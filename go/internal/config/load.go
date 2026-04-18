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
		pending := PendingConfigPath(name)
		if _, err := os.Stat(pending); err == nil {
			if err := loadFile(k, pending, allowInvalid); err != nil {
				return err
			}
			continue
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config: read %s: %w", pending, err)
		}

		desired := ConfigPath(name)
		if _, err := os.Stat(desired); err == nil {
			if err := loadFile(k, desired, allowInvalid); err != nil {
				return err
			}
			continue
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config: read %s: %w", desired, err)
		}

		live := LiveConfigPath(name)
		if _, err := os.Stat(live); err == nil {
			if err := loadFile(k, live, allowInvalid); err != nil {
				return err
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config: read %s: %w", live, err)
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
