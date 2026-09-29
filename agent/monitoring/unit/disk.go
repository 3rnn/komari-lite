package monitoring

import (
	"fmt"
	"strings"

	"github.com/nuomiiiii/lite-agent/runtimeconfig"
	"github.com/shirou/gopsutil/v4/disk"
)

type DiskInfo struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

func Disk() DiskInfo {
	diskinfo := DiskInfo{}
	// List all partitions; true prevents gopsutil from mistakenly excluding physical disks.
	usage, err := disk.Partitions(true)
	if err != nil {
		diskinfo.Total = 0
		diskinfo.Used = 0
	} else {
		// When custom mount points are specified, count only those points.
		includeMountpoints := runtimeconfig.IncludeMountpoints()
		if includeMountpoints != "" {
			includeMounts := strings.Split(includeMountpoints, ";")
			for _, mountpoint := range includeMounts {
				mountpoint = strings.TrimSpace(mountpoint)
				if mountpoint != "" {
					u, err := disk.Usage(mountpoint)
					if err != nil {
						continue
					} else {
						diskinfo.Total += u.Total
						diskinfo.Used += u.Used
					}
				}
			}
		} else {
			// By default, exclude temporary filesystems and network drives.
			deviceMap := make(map[string]*disk.UsageStat)

			for _, part := range usage {
				if isPhysicalDisk(part) {
					u, err := disk.Usage(part.Mountpoint)
					if err != nil {
						continue
					}

					deviceID := part.Device
					// Deduplicate ZFS by pool name (e.g. pool/dataset -> pool).
					if strings.ToLower(part.Fstype) == "zfs" {
						if idx := strings.Index(deviceID, "/"); idx != -1 {
							deviceID = deviceID[:idx]
						}
					}

					// If this device already exists but this mount has a larger Total, replace it (e.g. quota differences).
					// Otherwise keep the existing entry, usually the physical pool total.
					if existing, ok := deviceMap[deviceID]; ok {
						if u.Total > existing.Total {
							deviceMap[deviceID] = u
						}
					} else {
						deviceMap[deviceID] = u
					}
				}
			}

			for _, u := range deviceMap {
				diskinfo.Total += u.Total
				diskinfo.Used += u.Used
			}
		}
	}
	return diskinfo
}

// isPhysicalDisk determines whether a partition is a physical disk.
func isPhysicalDisk(part disk.PartitionStat) bool {
	// Always include the root mount for loop-based filesystems such as LXC.
	if part.Mountpoint == "/" {
		return true
	}
	mountpoint := strings.ToLower(part.Mountpoint)
	// Exclude mount points.
	var mountpointsToExcludePerfix = []string{
		"/tmp",
		"/var/tmp",
		"/dev",
		"/run",
		"/var/lib/containers",
		"/var/lib/docker",
		"/proc",
		"/sys",
		"/sys/fs/cgroup",
		"/etc/resolv.conf",
		"/etc/host", // /etc/hosts,/etc/hostname
		"/nix/store",
	}
	for _, mp := range mountpointsToExcludePerfix {
		if mountpoint == mp || strings.HasPrefix(mountpoint, mp) {
			return false
		}
	}

	fstype := strings.ToLower(part.Fstype)

	// Exclude Linux autofs triggers; the real filesystem appears as a separate partition.
	// Treating autofs as nonphysical prevents double-counting capacity.
	if fstype == "autofs" && !strings.HasPrefix(part.Device, "/dev/") {
		return false
	}

	// Include NTFS volumes mounted through ntfs-3g (fuseblk) on Linux; these are physical disks.
	if fstype == "fuseblk" {
		return true
	}
	var fstypeToExclude = []string{
		"tmpfs",
		"devtmpfs",
		"udev",
		"nfs",
		"cifs",
		"smb",
		"vboxsf",
		"9p",
		"fuse",
		"overlay",
		"proc",
		"devpts",
		"sysfs",
		"cgroup",
		"mqueue",
		"hugetlbfs",
		"debugfs",
		"binfmt_misc",
		"securityfs",
	}
	for _, fs := range fstypeToExclude {
		if fstype == fs || strings.HasPrefix(fstype, fs) {
			return false
		}
	}
	// Windows network drives are commonly mapped drive letters but are hard to identify by fstype.
	// Options can identify some Windows network drives.
	optsStr := strings.ToLower(strings.Join(part.Opts, ","))
	if strings.Contains(optsStr, "remote") || strings.Contains(optsStr, "network") {
		return false
	}

	// Virtual memory.
	if strings.HasPrefix(part.Device, "/dev/loop") {
		return false
	}

	return true
}

func DiskList() ([]string, error) {
	diskList := []string{}
	includeMountpoints := runtimeconfig.IncludeMountpoints()
	if includeMountpoints != "" {
		includeMounts := strings.Split(includeMountpoints, ";")
		for _, mountpoint := range includeMounts {
			mountpoint = strings.TrimSpace(mountpoint)
			if mountpoint != "" {
				diskList = append(diskList, mountpoint)
			}
		}
	} else {
		usage, err := disk.Partitions(true)
		if err != nil {
			return nil, err
		}

		// Keep only the shortest root mount path for the same physical device.
		deviceMap := make(map[string]disk.PartitionStat)
		for _, part := range usage {
			if isPhysicalDisk(part) {
				deviceID := part.Device
				// Deduplicate ZFS by pool name.
				if strings.ToLower(part.Fstype) == "zfs" {
					if idx := strings.Index(deviceID, "/"); idx != -1 {
						deviceID = deviceID[:idx]
					}
				}

				if existing, ok := deviceMap[deviceID]; ok {
					// Prefer shorter mount paths (e.g. /volume1 over /volume1/@appdata/...).
					if len(part.Mountpoint) < len(existing.Mountpoint) {
						deviceMap[deviceID] = part
					}
				} else {
					deviceMap[deviceID] = part
				}
			}
		}

		for _, part := range deviceMap {
			diskList = append(diskList, fmt.Sprintf("%s (%s)", part.Mountpoint, part.Fstype))
		}
	}
	return diskList, nil
}
