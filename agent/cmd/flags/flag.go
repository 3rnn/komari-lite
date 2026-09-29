package flags_pkg

type Config struct {
	AutoDiscoveryKey     string  `json:"auto_discovery_key" env:"AGENT_AUTO_DISCOVERY_KEY"`           // Deprecated: legacy marker to read auto-discovery.json; not a registration key
	DisableAutoUpdate    bool    `json:"disable_auto_update" env:"AGENT_DISABLE_AUTO_UPDATE"`         // Disable auto-update.
	RemoteControlEnabled bool    `json:"remote_control_enabled" env:"AGENT_REMOTE_CONTROL_ENABLED"`   // Deprecated: no remote control/terminal/exec/MCP in the lite Agent; parsed for compatibility.
	DisableWebSsh        bool    `json:"disable_web_ssh" env:"AGENT_DISABLE_WEB_SSH"`                 // Hidden migration input: legacy disable-remote-control flag.
	MemoryModeAvailable  bool    `json:"memory_mode_available" env:"AGENT_MEMORY_MODE_AVAILABLE"`     // Deprecated; use MemoryIncludeCache.
	Token                string  `json:"token" env:"AGENT_TOKEN"`                                     // Token
	Endpoint             string  `json:"endpoint" env:"AGENT_ENDPOINT"`                               // Panel URL.
	Interval             float64 `json:"interval" env:"AGENT_INTERVAL"`                               // Collection interval in seconds.
	IgnoreUnsafeCert     bool    `json:"ignore_unsafe_cert" env:"AGENT_IGNORE_UNSAFE_CERT"`           // Ignore invalid certificates.
	MaxRetries           int     `json:"max_retries" env:"AGENT_MAX_RETRIES"`                         // Maximum retries.
	ReconnectInterval    int     `json:"reconnect_interval" env:"AGENT_RECONNECT_INTERVAL"`           // Reconnect interval in seconds.
	InfoReportInterval   int     `json:"info_report_interval" env:"AGENT_INFO_REPORT_INTERVAL"`       // Basic info report interval in minutes.
	IncludeNics          string  `json:"include_nics" env:"AGENT_INCLUDE_NICS"`                       // Only count these comma-separated interfaces; wildcards supported.
	ExcludeNics          string  `json:"exclude_nics" env:"AGENT_EXCLUDE_NICS"`                       // Exclude these comma-separated interfaces; wildcards supported.
	IncludeMountpoints   string  `json:"include_mountpoints" env:"AGENT_INCLUDE_MOUNTPOINTS"`         // Only count these semicolon-separated mount points.
	MonthRotate          int     `json:"month_rotate" env:"AGENT_MONTH_ROTATE"`                       // Monthly traffic reset day (0 disables reset).
	MonthRotateTime      string  `json:"month_rotate_time" env:"AGENT_MONTH_ROTATE_TIME"`             // Reset time HH:MM:SS; empty means 00:00:00.
	MonthRotateTimezone  string  `json:"month_rotate_timezone" env:"AGENT_MONTH_ROTATE_TIMEZONE"`     // IANA timezone; empty means Asia/Shanghai.
	CFAccessClientID     string  `json:"cf_access_client_id" env:"AGENT_CF_ACCESS_CLIENT_ID"`         // Cloudflare Access Client ID
	CFAccessClientSecret string  `json:"cf_access_client_secret" env:"AGENT_CF_ACCESS_CLIENT_SECRET"` // Cloudflare Access Client Secret
	MemoryIncludeCache   bool    `json:"memory_include_cache" env:"AGENT_MEMORY_INCLUDE_CACHE"`       // Include cache/buffers in memory usage.
	MemoryReportRawUsed  bool    `json:"memory_report_raw_used" env:"AGENT_MEMORY_REPORT_RAW_USED"`   // Report raw used memory.
	CustomDNS            string  `json:"custom_dns" env:"AGENT_CUSTOM_DNS"`                           // Custom DNS server.
	EnableGPU            bool    `json:"enable_gpu" env:"AGENT_ENABLE_GPU"`                           // Enable detailed GPU monitoring.
	CustomIpv4           string  `json:"custom_ipv4" env:"AGENT_CUSTOM_IPV4"`                         // Custom IPv4 address.
	CustomIpv6           string  `json:"custom_ipv6" env:"AGENT_CUSTOM_IPV6"`                         // Custom IPv6 address.
	GetIpAddrFromNic     bool    `json:"get_ip_addr_from_nic" env:"AGENT_GET_IP_ADDR_FROM_NIC"`       // Get IP address from a network interface.
	HostProc             string  `json:"host_proc" env:"HOST_PROC"`                                   // Host /proc mount point for monitoring host processes in a container.
	ConfigFile           string  `json:"config_file" env:"AGENT_CONFIG_FILE"`                         // JSON configuration file path.
	ProtocolVersion      int     `json:"protocol_version" env:"AGENT_PROTOCOL_VERSION"`               // Reporting protocol version (only 2 supported).
	DisableCompression   bool    `json:"disable_compression" env:"AGENT_DISABLE_COMPRESSION"`         // Disable v2 transport compression.
	PreferIPVersion      string  `json:"prefer_ip_version" env:"AGENT_PREFER_IP_VERSION"`             // Preferred IP version for panel connections: 4 or 6.

}

var GlobalConfig = &Config{}

// RemoteControlEnabled reports the parsed legacy flag value.
//
// Deprecated: the slim Agent has no remote control, terminal, exec, file or MCP
// subsystems, so this value never enables anything. The flag is still parsed so
// that service units written by older installers keep starting.
func RemoteControlEnabled() bool {
	return GlobalConfig != nil && GlobalConfig.RemoteControlEnabled
}
