//go:build linux || windows

package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func serverStatePath(string) string {
	return filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
}

func loadServerStateDoc(path string) (map[string]any, error) {
	doc := make(map[string]any)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return doc, nil
		}
		return nil, fmt.Errorf("xp2p: read server config %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return doc, nil
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("xp2p: parse server config %s: %w", path, err)
	}
	raw := tree.GetPath([]string{"server"})
	switch value := raw.(type) {
	case *toml.Tree:
		doc = value.ToMap()
	case map[string]any:
		doc = value
	}
	return doc, nil
}

func writeServerStateDoc(path string, doc map[string]any) error {
	tree, err := loadOrCreateServerToml(path)
	if err != nil {
		return err
	}
	if doc == nil {
		doc = make(map[string]any)
	}
	existing := make(map[string]any)
	switch raw := tree.GetPath([]string{"server"}).(type) {
	case *toml.Tree:
		existing = raw.ToMap()
	case map[string]any:
		existing = raw
	}
	for key, value := range doc {
		existing[key] = value
	}
	tree.SetPath([]string{"server"}, existing)
	return writeServerTomlTree(path, tree)
}

func decodeServerReverseState(doc map[string]any) (serverReverseState, error) {
	raw := doc[serverReverseStateKey]
	if raw == nil {
		state := serverReverseState{}
		state.ensure()
		return state, nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("xp2p: encode server reverse state: %w", err)
	}
	var state serverReverseState
	if err := json.Unmarshal(buf, &state); err != nil {
		return nil, fmt.Errorf("xp2p: parse server reverse state: %w", err)
	}
	state.ensure()
	return state, nil
}

func loadOrCreateServerToml(path string) (*toml.Tree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tree, err := toml.TreeFromMap(map[string]any{})
			if err != nil {
				return nil, fmt.Errorf("xp2p: create empty server config tree: %w", err)
			}
			return tree, nil
		}
		return nil, fmt.Errorf("xp2p: read server config %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		tree, err := toml.TreeFromMap(map[string]any{})
		if err != nil {
			return nil, fmt.Errorf("xp2p: create empty server config tree: %w", err)
		}
		return tree, nil
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("xp2p: parse server config %s: %w", path, err)
	}
	return tree, nil
}

func writeServerTomlTree(path string, tree *toml.Tree) error {
	if tree == nil {
		return errors.New("xp2p: config tree is nil")
	}
	data, err := toml.Marshal(tree.ToMap())
	if err != nil {
		return fmt.Errorf("xp2p: encode server config %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure server config dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("xp2p: write server config %s: %w", path, err)
	}
	return nil
}
