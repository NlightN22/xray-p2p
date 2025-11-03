// Package config loads xp2p configuration from defaults, files, environment variables, and explicit overrides.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const defaultEnvPrefix = "XP2P_"

var defaultValues = map[string]any{
	"logging.level":         "info",
	"logging.format":        "text",
	"server.port":           "62022",
	"server.install_dir":    "",
	"server.config_dir":     "config-server",
	"server.mode":           "auto",
	"server.certificate":    "",
	"server.key":            "",
	"server.host":           "",
	"client.install_dir":    "",
	"client.config_dir":     "config-client",
	"client.server_address": "",
	"client.server_port":    "8443",
	"client.user":           "",
	"client.password":       "",
	"client.server_name":    "",
	"client.allow_insecure": true,
	"client.socks_address":  "127.0.0.1:51080",
}

var defaultCandidates = []string{
	"xp2p.yaml",
	"xp2p.yml",
	"xp2p.toml",
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

// ServerConfig holds diagnostics server settings.
type ServerConfig struct {
	Port            string `koanf:"port"`
	InstallDir      string `koanf:"install_dir"`
	ConfigDir       string `koanf:"config_dir"`
	Mode            string `koanf:"mode"`
	CertificateFile string `koanf:"certificate"`
	KeyFile         string `koanf:"key"`
	Host            string `koanf:"host"`
}

// ClientConfig holds client installation settings.
type ClientConfig struct {
	InstallDir    string `koanf:"install_dir"`
	ConfigDir     string `koanf:"config_dir"`
	ServerAddress string `koanf:"server_address"`
	ServerPort    string `koanf:"server_port"`
	User          string `koanf:"user"`
	Password      string `koanf:"password"`
	ServerName    string `koanf:"server_name"`
	AllowInsecure bool   `koanf:"allow_insecure"`
	SocksAddress  string `koanf:"socks_address"`
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
}

// Load constructs the configuration by merging defaults, optional file, environment, and overrides.
func Load(opts Options) (Config, error) {
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(defaultValues, "."), nil); err != nil {
		return Config{}, fmt.Errorf("config: load defaults: %w", err)
	}

	if err := loadFileIfPresent(k, opts.Path); err != nil {
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

func loadFileIfPresent(k *koanf.Koanf, explicitPath string) error {
	if explicitPath != "" {
		return loadFile(k, explicitPath)
	}

	for _, candidate := range defaultCandidates {
		if _, err := os.Stat(candidate); err == nil {
			return loadFile(k, candidate)
		}
	}
	return nil
}

func loadFile(k *koanf.Koanf, path string) error {
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
		return fmt.Errorf("config: parse %s: %w", path, err)
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

	// AllowInsecure is a boolean and defaults through the map loader.
}

func defaultInstallDir() string {
	if runtime.GOOS == "windows" {
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			return filepath.Join(pf, "xp2p")
		}
		return filepath.Join("C:\\", "xp2p")
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "xp2p")
	}

	return filepath.Join(os.TempDir(), "xp2p")
}
