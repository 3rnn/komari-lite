package config

import "time"

const (
	AdminDefaultPageSize    = 10
	AdminDefaultPageSizeMin = 5
	AdminDefaultPageSizeMax = 100
)

type Settings struct {
	ID                     uint   `json:"id,omitempty"`                                        // 1
	Sitename               string `json:"sitename" default:"Komari Lite"`                      // Site name, default "Komari Lite"
	Description            string `json:"description" default:"A simple server monitor tool."` // site description
	AdminDefaultPageSize   int    `json:"admin_default_page_size" default:"10"`                // The default number of items per page in the background list
	ReduceMotion           bool   `json:"reduce_motion" default:"false"`                       // Reduce background interface dynamic effects
	AutoOrderNewClients    bool   `json:"auto_order_new_clients_by_region" default:"false"`    // After the new server recognizes the country for the first time, it will automatically be ranked behind the nodes in the same group and the same country. It is turned off by default.
	CorsOriginCheckEnabled bool   `json:"cors_origin_check_enabled" default:"true"`            // Whether to enable API CORS cross-domain request verification, default true
	CorsAllowedOrigins     string `json:"cors_allowed_origins" default:""`                     // API cross-domain allow list
	WsOriginCheckEnabled   bool   `json:"ws_origin_check_enabled" default:"true"`              // Whether to verify WebSocket Origin
	WsAllowedOrigins       string `json:"ws_allowed_origins" default:""`                       // WebSocket Origin allow list
	Theme                  string `json:"theme" default:"nezha"`                               // New installations use the Nezha public theme by default
	PrivateSite            bool   `json:"private_site" default:"false"`                        // Whether it is a private site, default false
	ApiKey                 string `json:"api_key" default:""`                                  // API key, default empty string
	AutoDiscoveryKey       string `json:"auto_discovery_key" default:""`                       // Automatically discover keys
	ScriptDomain           string `json:"script_domain" default:""`                            // Custom script domain name
	SendIpAddrToGuest      bool   `json:"send_ip_addr_to_guest" default:"false"`               // Whether to send the IP address to the guest page, default false
	VisitorAuditEnabled    bool   `json:"visitor_audit_enabled" default:"false"`               // Whether to allow public guest events to be written to the audit log, default false
	EulaAccepted           bool   `json:"eula_accepted" default:"false"`
	BaseScriptsURLKey      string `json:"base_scripts_url" default:""`
	// GeoIP configuration
	GeoIpEnabled  bool   `json:"geo_ip_enabled" default:"false"`
	GeoIpProvider string `json:"geo_ip_provider" default:"ipinfo"` // empty, mmdb, ip-api, geojs
	// OAuth configuration
	OAuthEnabled          bool   `json:"o_auth_enabled" default:"false"`
	OAuthProvider         string `json:"o_auth_provider" default:"github"`
	DisablePasswordLogin  bool   `json:"disable_password_login" default:"false"`
	CloudflareTunnelToken string `json:"cloudflare_tunnel_token" default:""`
	HTTPSEnabled          bool   `json:"https_enabled" default:"false"`
	HTTPSListen           string `json:"https_listen" default:":35938"`
	HTTPSRedirectHTTP     bool   `json:"https_redirect_http" default:"false"`
	HTTPSCertificatePath  string `json:"https_certificate_path" default:"./data/tls/server.crt"`
	HTTPSPrivateKeyPath   string `json:"https_private_key_path" default:"./data/tls/server.key"`
	// Custom landscaping
	CustomHead string `json:"custom_head" default:""`
	CustomBody string `json:"custom_body" default:""`

	// Notification
	NotificationEnabled        bool    `json:"notification_enabled" default:"true"` // Notification master switch
	NotificationMethod         string  `json:"notification_method" default:"none"`
	NotificationTemplate       string  `json:"notification_template" default:"{{emoji}}{{emoji}}{{emoji}}\nEvent: {{event}}\nClients: {{client}}\nMessage: {{message}}\nTime: {{time}}"`
	ExpireNotificationEnabled  bool    `json:"expire_notification_enabled" default:"true"` // Whether to enable expiry notifications
	ExpireNotificationLeadDays int     `json:"expire_notification_lead_days" default:"7"`  // How many days to notify before expiration, the default is 7 days
	LoginNotification          bool    `json:"login_notification" default:"true"`          // Login notification
	TrafficLimitPercentage     float64 `json:"traffic_limit_percentage" default:"80.00"`   // Traffic limit percentage, default 80.00%
	TrafficReportTime          string  `json:"traffic_report_time" default:"00:00"`        // Traffic daily/weekly/monthly report sending time (Beijing time)
	UpdatedAt                  time.Time
}

const (
	SitenameKey               = "sitename"
	DescriptionKey            = "description"
	AdminDefaultPageSizeKey   = "admin_default_page_size"
	ReduceMotionKey           = "reduce_motion"
	AutoOrderNewClientsKey    = "auto_order_new_clients_by_region"
	CorsOriginCheckEnabledKey = "cors_origin_check_enabled"
	CorsAllowedOriginsKey     = "cors_allowed_origins"
	WsOriginCheckEnabledKey   = "ws_origin_check_enabled"
	WsAllowedOriginsKey       = "ws_allowed_origins"
	ThemeKey                  = "theme"
	PrivateSiteKey            = "private_site"
	ApiKeyKey                 = "api_key"
	AutoDiscoveryKeyKey       = "auto_discovery_key"
	ScriptDomainKey           = "script_domain"
	SendIpAddrToGuestKey      = "send_ip_addr_to_guest"
	VisitorAuditEnabledKey    = "visitor_audit_enabled"
	// LowResourceModeKey is retained only to normalize databases created by
	// releases that exposed the removed low-resource mode.
	LowResourceModeKey       = "low_resource_mode"
	EulaAcceptedKey          = "eula_accepted"
	BaseScriptsURLKey        = "base_scripts_url"
	GeoIpEnabledKey          = "geo_ip_enabled"
	GeoIpProviderKey         = "geo_ip_provider"
	OAuthEnabledKey          = "o_auth_enabled"
	OAuthProviderKey         = "o_auth_provider"
	DisablePasswordLoginKey  = "disable_password_login"
	CloudflareTunnelTokenKey = "cloudflare_tunnel_token"
	HTTPSEnabledKey          = "https_enabled"
	HTTPSListenKey           = "https_listen"
	HTTPSRedirectHTTPKey     = "https_redirect_http"
	HTTPSCertificatePathKey  = "https_certificate_path"
	HTTPSPrivateKeyPathKey   = "https_private_key_path"
	CustomHeadKey            = "custom_head"
	CustomBodyKey            = "custom_body"

	NotificationEnabledKey         = "notification_enabled"
	NotificationMethodKey          = "notification_method"
	NotificationTemplateKey        = "notification_template"
	OfflineNotificationDefaultKey  = "offline_notification_default"
	PingLossNotificationDefaultKey = "ping_loss_notification_default"
	TrafficReportDefaultKey        = "traffic_report_default"
	ExpireNotificationEnabledKey   = "expire_notification_enabled"
	ExpireNotificationLeadDaysKey  = "expire_notification_lead_days"
	LoginNotificationKey           = "login_notification"
	TrafficLimitPercentageKey      = "traffic_limit_percentage"
	TrafficReportTimeKey           = "traffic_report_time"
	UpdatedAtKey                   = "updated_at"
	DashboardSettingsKey           = "dashboard_settings"
)
