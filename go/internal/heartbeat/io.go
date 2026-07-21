package heartbeat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Load reads the heartbeat state from path without keeping it in memory.
func Load(path string) (State, error) {
	state, err := readState(path)
	mainMissing := errors.Is(err, os.ErrNotExist)
	if err != nil && !mainMissing {
		return State{}, err
	}
	state.ensure()
	if failure, failureErr := readState(persistenceFailurePath(path)); failureErr == nil {
		for key, entry := range failure.Entries {
			state.Entries[key] = entry
		}
	} else if !errors.Is(failureErr, os.ErrNotExist) {
		return State{}, failureErr
	} else if mainMissing {
		return State{}, err
	}
	return state, nil
}

// Save writes the provided state to the given path.
func Save(path string, state State) error {
	state.ensure()
	return writeState(path, state)
}

func persistenceFailurePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return path + ".persistence-error.json"
}

func readState(path string) (State, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return State{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return State{}, nil
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("heartbeat: parse %s: %w", path, err)
	}
	return state, nil
}

func writeState(path string, state State) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("heartbeat: ensure dir %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("heartbeat: encode state %s: %w", path, err)
	}
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "heartbeat-*.tmp")
	if err != nil {
		return fmt.Errorf("heartbeat: create temp %s: %w", path, err)
	}
	tmpName := tmpFile.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("heartbeat: write temp %s: %w", path, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("heartbeat: close temp %s: %w", path, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("heartbeat: chmod temp %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		if removeErr := os.Remove(path); removeErr == nil {
			if retryErr := os.Rename(tmpName, path); retryErr == nil {
				removeTmp = false
				return nil
			} else {
				return fmt.Errorf("heartbeat: rename temp %s: %w", path, retryErr)
			}
		}
		return fmt.Errorf("heartbeat: rename temp %s: %w", path, err)
	}
	removeTmp = false
	return nil
}
