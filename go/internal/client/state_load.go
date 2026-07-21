package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
)

func loadClientInstallState(path string) (clientInstallState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return clientInstallState{baseDigest: digestClientDesired(nil)}, nil
		}
		return clientInstallState{}, fmt.Errorf("read client config %s: %w", path, err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return clientInstallState{baseDigest: digestClientDesired(data)}, nil
	}

	tree, err := toml.LoadBytes(data)
	if err != nil {
		return clientInstallState{}, fmt.Errorf("%w: %s: %v", ErrClientConfigParse, path, err)
	}
	raw := tree.GetPath([]string{"client"})
	if raw == nil {
		state := clientInstallState{}
		state.normalize()
		state.baseDigest = digestClientDesired(data)
		return state, nil
	}
	switch value := raw.(type) {
	case *toml.Tree:
		raw = value.ToMap()
	case map[string]any:
		raw = value
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return clientInstallState{}, fmt.Errorf("encode client config %s: %w", path, err)
	}
	var state clientInstallState
	if err := json.Unmarshal(buf, &state); err != nil {
		return clientInstallState{}, fmt.Errorf("decode client config %s: %w", path, err)
	}
	state.normalize()
	if err := state.validateHeartbeatModes(); err != nil {
		return clientInstallState{}, fmt.Errorf("decode client config %s: %w", path, err)
	}
	state.baseDigest = digestClientDesired(data)
	return state, nil
}

func digestClientDesired(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func currentClientDesiredDigest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return digestClientDesired(nil), nil
		}
		return "", err
	}
	return digestClientDesired(data), nil
}

func loadClientInstallStateWithFallback(pendingPath, livePath string) (clientInstallState, error) {
	if strings.TrimSpace(pendingPath) != "" {
		if _, err := os.Stat(pendingPath); err == nil {
			return loadClientInstallState(pendingPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return clientInstallState{}, fmt.Errorf("stat client config %s: %w", pendingPath, err)
		}
	}
	return loadClientInstallState(livePath)
}
