package subscription

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrConcurrentStateChange = errors.New("subscription state changed concurrently")

type Store struct {
	Path string
}

func (s Store) Load() (PersistedSource, string, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PersistedSource{}, stateDigest(nil), nil
		}
		return PersistedSource{}, "", fmt.Errorf("read subscription state: %w", err)
	}
	var state PersistedSource
	if err := json.Unmarshal(data, &state); err != nil {
		return PersistedSource{}, "", fmt.Errorf("decode subscription state: %w", err)
	}
	return state, stateDigest(data), nil
}

func (s Store) Save(state PersistedSource, expectedDigest string) error {
	_, currentDigest, err := s.Load()
	if err != nil {
		return err
	}
	if expectedDigest != "" && currentDigest != expectedDigest {
		return ErrConcurrentStateChange
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode subscription state: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create subscription state directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".subscription-*.tmp")
	if err != nil {
		return fmt.Errorf("create subscription state temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.Path); err != nil {
		return fmt.Errorf("publish subscription state: %w", err)
	}
	return nil
}

func stateDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
