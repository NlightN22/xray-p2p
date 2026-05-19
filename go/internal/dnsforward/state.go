//go:build linux

package dnsforward

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func loadState(path string) (state, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state{}, nil
		}
		return state{}, fmt.Errorf("read dns-forward state %s: %w", path, err)
	}
	if len(data) == 0 {
		return state{}, nil
	}

	var raw rawState
	if err := json.Unmarshal(data, &raw); err != nil {
		return state{}, fmt.Errorf("parse dns-forward state %s: %w", path, err)
	}
	s, _, err := normalizeState(raw)
	if err != nil {
		return state{}, fmt.Errorf("normalize dns-forward state %s: %w", path, err)
	}
	if s.Entries == nil {
		s.Entries = make(map[string]stateEntry)
	}
	return s, nil
}

func (s *state) save(path string) error {
	if s.Entries == nil {
		s.Entries = make(map[string]stateEntry)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode dns-forward state %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure dns-forward state dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write dns-forward state %s: %w", path, err)
	}
	return nil
}

func (s *state) record(domain string, entry stateEntry) {
	if s.Entries == nil {
		s.Entries = make(map[string]stateEntry)
	}
	s.Entries[domain] = entry
}

func (s *state) remove(domain string) {
	if len(s.Entries) == 0 {
		return
	}
	delete(s.Entries, domain)
}
