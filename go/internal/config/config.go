// Package config loads xp2p configuration from defaults, files, environment variables, and explicit overrides.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const defaultEnvPrefix = "XP2P_"

var ErrConfigParse = errors.New("config: parse error")

var defaultValues = map[string]any{
	"logging.level":              "info",
	"logging.format":             "text",
	"server.port":                "62022",
	"server.trojan_port":         "58443",
	"server.install_dir":         "",
	"server.config_dir":          "config-server",
	"server.mode":                "auto",
	"server.cert_store":          "",
	"server.certificate":         "",
	"server.key":                 "",
	"server.host":                "",
	"server.tun_enabled":         true,
	"server.tun_name":            "xp2ps",
	"server.tun_mtu":             1500,
	"server.tun_addr":            "198.18.0.5/30",
	"client.install_dir":         "",
	"client.config_dir":          "config-client",
	"client.server_address":      "",
	"client.server_port":         "8443",
	"client.diag_port":           "62023",
	"client.user":                "",
	"client.password":            "",
	"client.server_name":         "",
	"client.allow_insecure":      false,
	"client.socks_address":       "127.0.0.1:51180",
	"client.tun_enabled":         true,
	"client.tun_name":            "xp2pc",
	"client.tun_mtu":             1500,
	"client.tun_addr":            "198.18.0.1/30",
	"client.tun_mode":            "split",
	"client.dns_servers":         []string{},
	"client.full_tunnel_verbose": false,
	"client.full_tunnel_tag":     "",
}

// Config represents the top-level application configuration.
type Config struct {
	Logging LoggingConfig `koanf:"logging"`
	Server  ServerConfig  `koanf:"server"`
	Client  ClientConfig  `koanf:"client"`
}

// LoggingConfig holds logging related settings.
type LoggingConfig struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

// ServerConfig holds server settings.
type ServerConfig struct {
	Port             string `koanf:"port"`
	TrojanPort       string `koanf:"trojan_port"`
	InstallDir       string `koanf:"install_dir"`
	ConfigDir        string `koanf:"config_dir"`
	Mode             string `koanf:"mode"`
	CertificateStore string `koanf:"cert_store"`
	CertificateFile  string `koanf:"certificate"`
	KeyFile          string `koanf:"key"`
	Host             string `koanf:"host"`
	TunEnabled       bool   `koanf:"tun_enabled"`
	TunName          string `koanf:"tun_name"`
	TunMTU           int    `koanf:"tun_mtu"`
	TunAddr          string `koanf:"tun_addr"`
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

// Load constructs the configuration by merging defaults, optional file, environment, and overrides.
func Load(opts Options) (Config, error) {
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(defaultValues, "."), nil); err != nil {
		return Config{}, fmt.Errorf("config: load defaults: %w", err)
	}

	if err := loadFileIfPresent(k, opts.Path, opts.AllowInvalid); err != nil {
		return Config{}, err
	}

	envPrefix := opts.EnvPrefix
	if envPrefix == "" {
		envPrefix = defaultEnvPrefix
	}

	if err := k.Load(env.Provider(envPrefix, ".", envKeyToPath(envPrefix)), nil); err != nil {
		return Config{}, fmt.Errorf("config: load environment: %w", err)
	}

	if len(opts.Overrides) > 0 {
		if err := k.Load(confmap.Provider(opts.Overrides, "."), nil); err != nil {
			return Config{}, fmt.Errorf("config: apply overrides: %w", err)
		}
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("config: decode: %w", err)
	}

	normalize(&cfg)

	return cfg, nil
}

func loadFileIfPresent(k *koanf.Koanf, explicitPath string, allowInvalid bool) error {
	if explicitPath != "" {
		return loadFile(k, explicitPath, allowInvalid)
	}

	roleFiles := []string{
		layout.ClientConfigFileName,
		layout.ServerConfigFileName,
	}
	for _, name := range roleFiles {
		live := ConfigPath(name)
		if _, err := os.Stat(live); err == nil {
			if err := loadFile(k, live, allowInvalid); err != nil {
				return err
			}
			continue
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config: read %s: %w", live, err)
		}

		pending := PendingConfigPath(name)
		if _, err := os.Stat(pending); err == nil {
			if err := loadFile(k, pending, allowInvalid); err != nil {
				return err
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config: read %s: %w", pending, err)
		}
	}
	return nil
}

func loadFile(k *koanf.Koanf, path string, allowInvalid bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("config: read %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("config: %s is a directory", path)
	}

	parser, err := parserFor(path)
	if err != nil {
		return err
	}

	if err := k.Load(file.Provider(path), parser); err != nil {
		if allowInvalid {
			return nil
		}
		return fmt.Errorf("%w: %s: %v", ErrConfigParse, path, err)
	}
	return nil
}

func parserFor(path string) (koanf.Parser, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return yaml.Parser(), nil
	case ".toml":
		return toml.Parser(), nil
	default:
		return nil, fmt.Errorf("config: unsupported file format %s", filepath.Ext(path))
	}
}

func envKeyToPath(prefix string) func(string) string {
	return func(key string) string {
		key = strings.TrimPrefix(key, prefix)
		if key == "" {
			return ""
		}

		parts := strings.Split(key, "_")
		for i := range parts {
			parts[i] = strings.ToLower(parts[i])
		}

		if len(parts) == 1 {
			return parts[0]
		}

		return parts[0] + "." + strings.Join(parts[1:], "_")
	}
}

func normalize(cfg *Config) {
	cfg.Logging.Level = strings.TrimSpace(strings.ToLower(cfg.Logging.Level))
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = defaultValues["logging.level"].(string)
	}
	cfg.Logging.Format = strings.TrimSpace(strings.ToLower(cfg.Logging.Format))
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = defaultValues["logging.format"].(string)
	}

	cfg.Server.Port = strings.TrimSpace(cfg.Server.Port)
	if cfg.Server.Port == "" {
		cfg.Server.Port = defaultValues["server.port"].(string)
	}
	cfg.Server.TrojanPort = strings.TrimSpace(cfg.Server.TrojanPort)
	if cfg.Server.TrojanPort == "" {
		cfg.Server.TrojanPort = defaultValues["server.trojan_port"].(string)
	}

	cfg.Server.InstallDir = strings.TrimSpace(cfg.Server.InstallDir)
	if cfg.Server.InstallDir == "" {
		cfg.Server.InstallDir = defaultInstallDir()
	}

	cfg.Server.ConfigDir = strings.TrimSpace(cfg.Server.ConfigDir)
	if cfg.Server.ConfigDir == "" {
		cfg.Server.ConfigDir = defaultValues["server.config_dir"].(string)
	}

	cfg.Server.Mode = strings.TrimSpace(strings.ToLower(cfg.Server.Mode))
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = defaultValues["server.mode"].(string)
	}

	cfg.Server.CertificateStore = strings.TrimSpace(cfg.Server.CertificateStore)
	if cfg.Server.CertificateStore == "" {
		cfg.Server.CertificateStore = defaultValues["server.cert_store"].(string)
	}

	cfg.Server.CertificateFile = strings.TrimSpace(cfg.Server.CertificateFile)
	if cfg.Server.CertificateFile == "" {
		cfg.Server.CertificateFile = defaultValues["server.certificate"].(string)
	}

	cfg.Server.KeyFile = strings.TrimSpace(cfg.Server.KeyFile)
	if cfg.Server.KeyFile == "" {
		cfg.Server.KeyFile = defaultValues["server.key"].(string)
	}

	cfg.Server.Host = strings.TrimSpace(cfg.Server.Host)
	if cfg.Server.Host == "" {
		cfg.Server.Host = defaultValues["server.host"].(string)
	}

	cfg.Server.TunName = strings.TrimSpace(cfg.Server.TunName)
	if cfg.Server.TunName == "" {
		cfg.Server.TunName = defaultValues["server.tun_name"].(string)
	}

	if cfg.Server.TunMTU <= 0 {
		cfg.Server.TunMTU = defaultValues["server.tun_mtu"].(int)
	}

	cfg.Server.TunAddr = strings.TrimSpace(cfg.Server.TunAddr)
	if cfg.Server.TunAddr == "" {
		cfg.Server.TunAddr = defaultValues["server.tun_addr"].(string)
	}

	cfg.Client.InstallDir = strings.TrimSpace(cfg.Client.InstallDir)
	if cfg.Client.InstallDir == "" {
		cfg.Client.InstallDir = defaultInstallDir()
	}

	cfg.Client.ConfigDir = strings.TrimSpace(cfg.Client.ConfigDir)
	if cfg.Client.ConfigDir == "" {
		cfg.Client.ConfigDir = defaultValues["client.config_dir"].(string)
	}

	cfg.Client.ServerAddress = strings.TrimSpace(cfg.Client.ServerAddress)
	if cfg.Client.ServerAddress == "" {
		cfg.Client.ServerAddress = defaultValues["client.server_address"].(string)
	}

	cfg.Client.ServerPort = strings.TrimSpace(cfg.Client.ServerPort)
	if cfg.Client.ServerPort == "" {
		cfg.Client.ServerPort = defaultValues["client.server_port"].(string)
	}

	cfg.Client.DiagPort = strings.TrimSpace(cfg.Client.DiagPort)
	if cfg.Client.DiagPort == "" {
		cfg.Client.DiagPort = defaultValues["client.diag_port"].(string)
	}

	cfg.Client.User = strings.TrimSpace(cfg.Client.User)
	if cfg.Client.User == "" {
		cfg.Client.User = defaultValues["client.user"].(string)
	}

	cfg.Client.Password = strings.TrimSpace(cfg.Client.Password)
	if cfg.Client.Password == "" {
		cfg.Client.Password = defaultValues["client.password"].(string)
	}

	cfg.Client.ServerName = strings.TrimSpace(cfg.Client.ServerName)
	if cfg.Client.ServerName == "" {
		cfg.Client.ServerName = defaultValues["client.server_name"].(string)
	}

	cfg.Client.SocksAddress = strings.TrimSpace(cfg.Client.SocksAddress)
	if cfg.Client.SocksAddress == "" {
		cfg.Client.SocksAddress = defaultValues["client.socks_address"].(string)
	}

	cfg.Client.TunName = strings.TrimSpace(cfg.Client.TunName)
	if cfg.Client.TunName == "" {
		cfg.Client.TunName = defaultValues["client.tun_name"].(string)
	}

	if cfg.Client.TunMTU <= 0 {
		cfg.Client.TunMTU = defaultValues["client.tun_mtu"].(int)
	}

	cfg.Client.TunAddr = strings.TrimSpace(cfg.Client.TunAddr)
	if cfg.Client.TunAddr == "" {
		cfg.Client.TunAddr = defaultValues["client.tun_addr"].(string)
	}
	cfg.Client.TunMode = normalizeTunMode(cfg.Client.TunMode)
	cfg.Client.DNSServers = normalizeDNSServers(cfg.Client.DNSServers)
	cfg.Client.FullTunnelTag = strings.TrimSpace(cfg.Client.FullTunnelTag)

	// AllowInsecure is a boolean and defaults through the map loader.
}

func normalizeTunMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "full":
		return "full"
	case "split":
		return "split"
	default:
		return defaultValues["client.tun_mode"].(string)
	}
}

func normalizeDNSServers(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean == "" {
			continue
		}
		key := strings.ToLower(clean)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		trimmed = append(trimmed, clean)
	}
	if len(trimmed) == 0 {
		return []string{}
	}
	return trimmed
}
