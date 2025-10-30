// Package config loads xp2p configuration from defaults, files, environment variables, and explicit overrides.
package config

import (
	"fmt"
	"os"
	"path/filepath"
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
	"logging.level": "info",
	"server.port":   "62022",
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
}

// LoggingConfig holds logging related settings.
type LoggingConfig struct {
	Level string `koanf:"level"`
}

// ServerConfig holds diagnostics server settings.
type ServerConfig struct {
	Port string `koanf:"port"`
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
		key = strings.ReplaceAll(key, "_", ".")
		return strings.ToLower(key)
	}
}

func normalize(cfg *Config) {
	cfg.Logging.Level = strings.TrimSpace(strings.ToLower(cfg.Logging.Level))
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = defaultValues["logging.level"].(string)
	}

	cfg.Server.Port = strings.TrimSpace(cfg.Server.Port)
	if cfg.Server.Port == "" {
		cfg.Server.Port = defaultValues["server.port"].(string)
	}
}
