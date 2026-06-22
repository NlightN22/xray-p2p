// Package config loads xp2p configuration from defaults, files, environment variables, and explicit overrides.
package config

import "errors"

const defaultEnvPrefix = "XP2P_"

var ErrConfigParse = errors.New("config: parse error")

var defaultValues = map[string]any{
	"logging.level":                          "info",
	"logging.format":                         "text",
	"server.port":                            "62022",
	"server.trojan_port":                     "58443",
	"server.profile":                         "trojan-tls",
	"server.install_dir":                     "",
	"server.config_dir":                      "config-server",
	"server.mode":                            "auto",
	"server.cert_store":                      "",
	"server.certificate":                     "",
	"server.key":                             "",
	"server.host":                            "",
	"server.tun_enabled":                     false,
	"server.tun_name":                        "xp2ps",
	"server.tun_mtu":                         1500,
	"server.tun_addr":                        "198.18.0.5/30",
	"server.identity_provider.interval":      "15m",
	"server.identity_provider.max_cache_age": "24h",
	"client.install_dir":                     "",
	"client.config_dir":                      "config-client",
	"client.server_address":                  "",
	"client.server_port":                     "8443",
	"client.diag_port":                       "62023",
	"client.user":                            "",
	"client.password":                        "",
	"client.server_name":                     "",
	"client.allow_insecure":                  false,
	"client.socks_address":                   "127.0.0.1:51180",
	"client.tun_enabled":                     true,
	"client.tun_name":                        "xp2pc",
	"client.tun_mtu":                         1500,
	"client.tun_addr":                        "198.18.0.1/30",
	"client.tun_mode":                        "split",
	"client.dns_servers":                     []string{},
	"client.full_tunnel_verbose":             false,
	"client.full_tunnel_tag":                 "",
}

// Config represents the top-level application configuration.
type Config struct {
	Logging    LoggingConfig    `koanf:"logging"`
	Server     ServerConfig     `koanf:"server"`
	Client     ClientConfig     `koanf:"client"`
	XrayAssets XrayAssetsConfig `koanf:"xray_assets"`
}

// LoggingConfig holds logging related settings.
type LoggingConfig struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

// ServerConfig holds server settings.
type ServerConfig struct {
	Port             string                 `koanf:"port"`
	TrojanPort       string                 `koanf:"trojan_port"`
	Profile          string                 `koanf:"profile"`
	InstallDir       string                 `koanf:"install_dir"`
	ConfigDir        string                 `koanf:"config_dir"`
	Mode             string                 `koanf:"mode"`
	CertificateStore string                 `koanf:"cert_store"`
	CertificateFile  string                 `koanf:"certificate"`
	KeyFile          string                 `koanf:"key"`
	Host             string                 `koanf:"host"`
	TunEnabled       bool                   `koanf:"tun_enabled"`
	TunName          string                 `koanf:"tun_name"`
	TunMTU           int                    `koanf:"tun_mtu"`
	TunAddr          string                 `koanf:"tun_addr"`
	IdentityProvider IdentityProviderConfig `koanf:"identity_provider"`
}

// IdentityProviderConfig holds optional server directory provider settings.
type IdentityProviderConfig struct {
	InstanceID  string             `koanf:"instance_id" json:"instance_id,omitempty"`
	Kind        string             `koanf:"kind" json:"kind,omitempty"`
	Secret      string             `koanf:"secret" json:"secret,omitempty"`
	Interval    string             `koanf:"interval" json:"interval,omitempty"`
	MaxCacheAge string             `koanf:"max_cache_age" json:"max_cache_age,omitempty"`
	GroupIDs    []string           `koanf:"group_ids" json:"group_ids,omitempty"`
	LDAP        LDAPProviderConfig `koanf:"ldap" json:"ldap,omitempty"`
	SCIM        SCIMProviderConfig `koanf:"scim" json:"scim,omitempty"`
}

// LDAPProviderConfig describes one LDAP directory source.
type LDAPProviderConfig struct {
	URL                  string `koanf:"url" json:"url,omitempty"`
	BaseDN               string `koanf:"base_dn" json:"base_dn,omitempty"`
	BindDN               string `koanf:"bind_dn" json:"bind_dn,omitempty"`
	UserFilter           string `koanf:"user_filter" json:"user_filter,omitempty"`
	GroupFilter          string `koanf:"group_filter" json:"group_filter,omitempty"`
	SubjectAttribute     string `koanf:"subject_attribute" json:"subject_attribute,omitempty"`
	MembershipAttribute  string `koanf:"membership_attribute" json:"membership_attribute,omitempty"`
	DisplayNameAttribute string `koanf:"display_name_attribute" json:"display_name_attribute,omitempty"`
	PageSize             int    `koanf:"page_size" json:"page_size,omitempty"`
	Timeout              string `koanf:"timeout" json:"timeout,omitempty"`
	InsecureTLS          bool   `koanf:"insecure_tls" json:"insecure_tls,omitempty"`
}

// SCIMProviderConfig describes one SCIM directory source.
type SCIMProviderConfig struct {
	Endpoint    string `koanf:"endpoint" json:"endpoint,omitempty"`
	Token       string `koanf:"token" json:"token,omitempty"`
	Timeout     string `koanf:"timeout" json:"timeout,omitempty"`
	PageSize    int    `koanf:"page_size" json:"page_size,omitempty"`
	InsecureTLS bool   `koanf:"insecure_tls" json:"insecure_tls,omitempty"`
}

// ClientConfig holds client installation settings.
type ClientConfig struct {
	InstallDir        string   `koanf:"install_dir"`
	ConfigDir         string   `koanf:"config_dir"`
	ServerAddress     string   `koanf:"server_address"`
	ServerPort        string   `koanf:"server_port"`
	DiagPort          string   `koanf:"diag_port"`
	User              string   `koanf:"user"`
	Password          string   `koanf:"password"`
	ServerName        string   `koanf:"server_name"`
	AllowInsecure     bool     `koanf:"allow_insecure"`
	SocksAddress      string   `koanf:"socks_address"`
	TunEnabled        bool     `koanf:"tun_enabled"`
	TunName           string   `koanf:"tun_name"`
	TunMTU            int      `koanf:"tun_mtu"`
	TunAddr           string   `koanf:"tun_addr"`
	TunMode           string   `koanf:"tun_mode"`
	DNSServers        []string `koanf:"dns_servers"`
	FullTunnelVerbose bool     `koanf:"full_tunnel_verbose"`
	FullTunnelTag     string   `koanf:"full_tunnel_tag"`
}

// XrayAssetsConfig holds managed xray-core .dat asset settings.
type XrayAssetsConfig struct {
	StaleAfter string            `koanf:"stale_after" json:"stale_after,omitempty"`
	Files      []XrayAssetConfig `koanf:"files" json:"files,omitempty"`
}

// XrayAssetConfig describes one managed xray-core .dat asset.
type XrayAssetConfig struct {
	Name       string `koanf:"name" json:"name"`
	URL        string `koanf:"url,omitempty" json:"url,omitempty"`
	StaleAfter string `koanf:"stale_after,omitempty" json:"stale_after,omitempty"`
}

// Options control configuration loading behaviour.
type Options struct {
	// Path points to an explicit configuration file. When empty, the loader
	// searches defaultCandidates and loads the first match.
	Path string
	// EnvPrefix allows overriding the environment variable prefix (default XP2P_).
	EnvPrefix string
	// Overrides contains final in-memory values applied after all other sources.
	Overrides map[string]any
	// AllowInvalid allows ignoring configuration parse errors.
	AllowInvalid bool
}
