package xrayconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/pelletier/go-toml"
)

func toMap(cfg any) (map[string]any, error) {
	buf, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("xrayconfig: encode config: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf, &out); err != nil {
		return nil, fmt.Errorf("xrayconfig: decode config: %w", err)
	}
	return out, nil
}

func loadOrCreateToml(path string) (*toml.Tree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tree, err := toml.TreeFromMap(map[string]any{})
			if err != nil {
				return nil, fmt.Errorf("xrayconfig: create empty config tree: %w", err)
			}
			return tree, nil
		}
		return nil, fmt.Errorf("xrayconfig: read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		tree, err := toml.TreeFromMap(map[string]any{})
		if err != nil {
			return nil, fmt.Errorf("xrayconfig: create empty config tree: %w", err)
		}
		return tree, nil
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfigParse, path, err)
	}
	return tree, nil
}

func loadExistingToml(path string) (*toml.Tree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrConfigMissing, path)
		}
		return nil, fmt.Errorf("xrayconfig: read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrConfigEmpty, path)
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfigParse, path, err)
	}
	return tree, nil
}
