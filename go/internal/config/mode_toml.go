package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
)

func loadOrCreateModeToml(path string, role string) (*toml.Tree, error) {
	if _, err := os.Stat(path); err == nil {
		return loadOrCreateToml(path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	return toml.TreeFromMap(map[string]any{})
}

func loadOrCreateToml(path string) (*toml.Tree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tree, err := toml.TreeFromMap(map[string]any{})
			if err != nil {
				return nil, fmt.Errorf("config: create empty toml tree: %w", err)
			}
			return tree, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		tree, err := toml.TreeFromMap(map[string]any{})
		if err != nil {
			return nil, fmt.Errorf("config: create empty toml tree: %w", err)
		}
		return tree, nil
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfigParse, path, err)
	}
	return tree, nil
}

func encodeToml(tree *toml.Tree) ([]byte, error) {
	if tree == nil {
		return nil, errors.New("config: toml tree is nil")
	}
	data, err := toml.Marshal(tree.ToMap())
	if err != nil {
		return nil, fmt.Errorf("config: encode toml: %w", err)
	}
	return data, nil
}
