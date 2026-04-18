package client

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
)

func loadOrCreateToml(path string) (*toml.Tree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tree, err := toml.TreeFromMap(map[string]any{})
			if err != nil {
				return nil, fmt.Errorf("create empty client config tree: %w", err)
			}
			return tree, nil
		}
		return nil, fmt.Errorf("read client config %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		tree, err := toml.TreeFromMap(map[string]any{})
		if err != nil {
			return nil, fmt.Errorf("create empty client config tree: %w", err)
		}
		return tree, nil
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parse client config %s: %w", path, err)
	}
	return tree, nil
}

func writeTomlTree(path string, tree *toml.Tree) error {
	if tree == nil {
		return errors.New("config tree is nil")
	}
	data, err := toml.Marshal(tree.ToMap())
	if err != nil {
		return fmt.Errorf("encode client config %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure client config dir %s: %w", filepath.Dir(path), err)
	}
	if err := configio.WriteBytes(path, data, configio.WriteOptions{
		AuditPath: config.AuditLogPath(),
	}); err != nil {
		return err
	}
	return nil
}
