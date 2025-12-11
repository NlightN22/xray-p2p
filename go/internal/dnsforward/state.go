//go:build linux

package dnsforward

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type state struct {
	Entries map[string]stateEntry `json:"entries,omitempty"`
}

type stateEntry struct {
	Target            string `json:"target"`
	Server            string `json:"server"`
	ForwardListenPort int    `json:"forward_listen_port,omitempty"`
	ForwardTag        string `json:"forward_tag,omitempty"`
	AutoForward       bool   `json:"auto_forward,omitempty"`
	RebindDomain      string `json:"rebind_domain,omitempty"`
}

func loadState(path string) (state, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state{}, nil
		}
		return state{}, fmt.Errorf("xp2p: read dns-forward state %s: %w", path, err)
	}
	if len(data) == 0 {
		return state{}, nil
	}

	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return state{}, fmt.Errorf("xp2p: parse dns-forward state %s: %w", path, err)
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
		return fmt.Errorf("xp2p: encode dns-forward state %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure dns-forward state dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("xp2p: write dns-forward state %s: %w", path, err)
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
