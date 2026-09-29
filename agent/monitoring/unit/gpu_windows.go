//go:build windows
// +build windows

package monitoring

import (
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// GPUInfo stores details obtained for DXGI graphics adapters.
type GPUInfo struct {
	Index                 int    // DXGI adapter index.
	Name                  string // Adapter name.
	VendorId              uint32 // Vendor ID, e.g. 0x10DE NVIDIA, 0x1002 AMD, 0x8086 Intel.
	DeviceId              uint32 // Device ID.
	SubSysId              uint32 // Subsystem ID, usually including OEM information.
	Revision              uint32 // Hardware revision.
	DedicatedVideoMemory  uint64 // Dedicated VRAM (bytes).
	DedicatedSystemMemory uint64 // Dedicated system memory for the GPU (bytes).
	SharedSystemMemory    uint64 // Shared system memory (bytes).
	LUIDHighPart          int32  // High part of the locally unique identifier.
	LUIDLowPart           uint32 // Low part of the locally unique identifier.
	Flags                 uint32 // Adapter flags.
}

func (g *GPUInfo) DedicatedVideoMemoryMB() uint64 {
	return g.DedicatedVideoMemory / (1024 * 1024)
}

func (g *GPUInfo) DedicatedSystemMemoryMB() uint64 {
	return g.DedicatedSystemMemory / (1024 * 1024)
}

func (g *GPUInfo) SharedSystemMemoryMB() uint64 {
	return g.SharedSystemMemory / (1024 * 1024)
}

func (g *GPUInfo) LUIDString() string {
	return fmt.Sprintf("0x%08X%08X", uint32(g.LUIDHighPart), g.LUIDLowPart)
}

const (
	sOK               = uintptr(0x00000000)
	ePointer          = uintptr(0x80004003)
	dxgiErrorNotFound = uintptr(0x887A0002)
)

var iidIDXGIFactory1 = guid{
	Data1: 0x770AAE78,
	Data2: 0xF26F,
	Data3: 0x4DBA,
	Data4: [8]byte{0xA8, 0x29, 0x25, 0x3C, 0x83, 0xD1, 0xB3, 0x87},
}

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type luid struct {
	LowPart  uint32
	HighPart int32
}

// Corresponds to DXGI_ADAPTER_DESC1.
type dxgiAdapterDesc1 struct {
	Description           [128]uint16
	VendorId              uint32
	DeviceId              uint32
	SubSysId              uint32
	Revision              uint32
	DedicatedVideoMemory  uintptr
	DedicatedSystemMemory uintptr
	SharedSystemMemory    uintptr
	AdapterLuid           luid
	Flags                 uint32
}

func (d *dxgiAdapterDesc1) DescriptionString() string {
	end := len(d.Description)
	for i, v := range d.Description {
		if v == 0 {
			end = i
			break
		}
	}
	return string(utf16.Decode(d.Description[:end]))
}

type idxgiFactory1 struct {
	Vtbl *idxgiFactory1Vtbl
}

type idxgiFactory1Vtbl struct {
	QueryInterface          uintptr
	AddRef                  uintptr
	Release                 uintptr
	SetPrivateData          uintptr
	SetPrivateDataInterface uintptr
	GetPrivateData          uintptr
	GetParent               uintptr
	EnumAdapters            uintptr
	MakeWindowAssociation   uintptr
	GetWindowAssociation    uintptr
	CreateSwapChain         uintptr
	CreateSoftwareAdapter   uintptr
	EnumAdapters1           uintptr
	IsCurrent               uintptr
}

type idxgiAdapter1 struct {
	Vtbl *idxgiAdapter1Vtbl
}

type idxgiAdapter1Vtbl struct {
	QueryInterface          uintptr
	AddRef                  uintptr
	Release                 uintptr
	SetPrivateData          uintptr
	SetPrivateDataInterface uintptr
	GetPrivateData          uintptr
	GetParent               uintptr
	EnumOutputs             uintptr
	GetDesc                 uintptr
	CheckInterfaceSupport   uintptr
	GetDesc1                uintptr
}

func (f *idxgiFactory1) EnumAdapters1(adapterIndex uint32, ppAdapter **idxgiAdapter1) uintptr {
	if f == nil || f.Vtbl == nil || f.Vtbl.EnumAdapters1 == 0 || ppAdapter == nil {
		return ePointer
	}

	ret, _, _ := syscall.SyscallN(
		f.Vtbl.EnumAdapters1,
		uintptr(unsafe.Pointer(f)),
		uintptr(adapterIndex),
		uintptr(unsafe.Pointer(ppAdapter)),
	)
	return ret
}

func (a *idxgiAdapter1) GetDesc1(pDesc *dxgiAdapterDesc1) uintptr {
	if a == nil || a.Vtbl == nil || a.Vtbl.GetDesc1 == 0 || pDesc == nil {
		return ePointer
	}

	ret, _, _ := syscall.SyscallN(
		a.Vtbl.GetDesc1,
		uintptr(unsafe.Pointer(a)),
		uintptr(unsafe.Pointer(pDesc)),
	)
	return ret
}

func (f *idxgiFactory1) Release() uint32 {
	if f == nil || f.Vtbl == nil || f.Vtbl.Release == 0 {
		return 0
	}

	ret, _, _ := syscall.SyscallN(
		f.Vtbl.Release,
		uintptr(unsafe.Pointer(f)),
	)
	return uint32(ret)
}

func (a *idxgiAdapter1) Release() uint32 {
	if a == nil || a.Vtbl == nil || a.Vtbl.Release == 0 {
		return 0
	}

	ret, _, _ := syscall.SyscallN(
		a.Vtbl.Release,
		uintptr(unsafe.Pointer(a)),
	)
	return uint32(ret)
}

// GetGPUs enumerates and returns all DXGI graphics adapters in the system.
func GetGPUs() ([]GPUInfo, error) {
	dxgiDLL, err := syscall.LoadDLL("dxgi.dll")
	if err != nil {
		return nil, fmt.Errorf("load dxgi.dll failed: %w", err)
	}
	defer dxgiDLL.Release()

	procCreateDXGIFactory1, err := dxgiDLL.FindProc("CreateDXGIFactory1")
	if err != nil {
		return nil, fmt.Errorf("find CreateDXGIFactory1 failed: %w", err)
	}

	var factory *idxgiFactory1
	ret, _, _ := syscall.SyscallN(
		procCreateDXGIFactory1.Addr(),
		uintptr(unsafe.Pointer(&iidIDXGIFactory1)),
		uintptr(unsafe.Pointer(&factory)),
	)

	if ret != sOK {
		if factory != nil {
			factory.Release()
		}
		return nil, fmt.Errorf("CreateDXGIFactory1 failed with HRESULT: 0x%08X", uint32(ret))
	}

	if factory == nil {
		return nil, fmt.Errorf("CreateDXGIFactory1 succeeded but returned nil factory")
	}
	defer factory.Release()

	var gpus []GPUInfo

	for adapterIndex := uint32(0); ; adapterIndex++ {
		var adapter *idxgiAdapter1

		hr := factory.EnumAdapters1(adapterIndex, &adapter)
		if hr == dxgiErrorNotFound {
			break
		}

		if hr != sOK {
			if adapter != nil {
				adapter.Release()
			}
			return gpus, fmt.Errorf(
				"EnumAdapters1 failed at index %d with HRESULT: 0x%08X",
				adapterIndex,
				uint32(hr),
			)
		}

		if adapter == nil {
			return gpus, fmt.Errorf(
				"EnumAdapters1 succeeded but returned nil adapter at index %d",
				adapterIndex,
			)
		}

		var desc dxgiAdapterDesc1
		hr = adapter.GetDesc1(&desc)

		adapter.Release()

		if hr != sOK {
			return gpus, fmt.Errorf(
				"GetDesc1 failed at index %d with HRESULT: 0x%08X",
				adapterIndex,
				uint32(hr),
			)
		}

		info := GPUInfo{
			Index:                 int(adapterIndex),
			Name:                  desc.DescriptionString(),
			VendorId:              desc.VendorId,
			DeviceId:              desc.DeviceId,
			SubSysId:              desc.SubSysId,
			Revision:              desc.Revision,
			DedicatedVideoMemory:  uint64(desc.DedicatedVideoMemory),
			DedicatedSystemMemory: uint64(desc.DedicatedSystemMemory),
			SharedSystemMemory:    uint64(desc.SharedSystemMemory),
			LUIDHighPart:          desc.AdapterLuid.HighPart,
			LUIDLowPart:           desc.AdapterLuid.LowPart,
			Flags:                 desc.Flags,
		}

		gpus = append(gpus, info)
	}

	return gpus, nil
}

func GpuName() string {
	gpus, err := GetGPUs()
	if err != nil {
		return "Unknown"
	}

	type GPUKey struct {
		LUIDHighPart int32
		LUIDLowPart  uint32
	}
	seenGPUs := make(map[GPUKey]struct{})
	var names []string

	for _, gpu := range gpus {
		if (gpu.Flags & (1 << 1)) != 0 { // DXGI_ADAPTER_FLAG_SOFTWARE
			continue
		}

		key := GPUKey{
			LUIDHighPart: gpu.LUIDHighPart,
			LUIDLowPart:  gpu.LUIDLowPart,
		}

		if _, seen := seenGPUs[key]; !seen {
			seenGPUs[key] = struct{}{}
			name := gpu.Name
			if name != "" {
				names = append(names, name)
			}
		}
	}
	return formatGPUNameList(names)
}
