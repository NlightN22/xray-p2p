package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/installstate"
)

func serverStatePath(installDir string) string {
	return filepath.Join(installDir, installstate.FileNameForKind(installstate.KindServer))
}

func loadServerStateDoc(path string) (map[string]any, error) {
	doc := make(map[string]any)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return doc, nil
		}
		return nil, fmt.Errorf("xp2p: read server state %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("xp2p: parse server state %s: %w", path, err)
	}
	return doc, nil
}

func writeServerStateDoc(path string, doc map[string]any) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("xp2p: encode server state %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure server state dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("xp2p: write server state %s: %w", path, err)
	}
	return nil
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
