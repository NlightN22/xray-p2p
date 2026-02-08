package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/version"
)

type serverAppliedState struct {
	Reverse    serverReverseState `json:"reverse_channels"`
	Redirects  []redirect.Rule    `json:"server_redirects"`
	Forwards   []forward.Rule     `json:"forward_rules"`
	TunEnabled bool               `json:"tun_enabled"`
	TunName    string             `json:"tun_name"`
	TunMTU     int                `json:"tun_mtu"`
	TunAddr    string             `json:"tun_addr"`
	Mode       string             `json:"mode"`
	Version    string             `json:"version"`
	Timestamp  time.Time          `json:"timestamp"`
}

func loadServerAppliedState(path string) (serverAppliedState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return serverAppliedState{}, nil
		}
		return serverAppliedState{}, fmt.Errorf("xp2p: read server state %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return serverAppliedState{}, nil
	}
	var state serverAppliedState
	if err := json.Unmarshal(data, &state); err != nil {
		return serverAppliedState{}, fmt.Errorf("xp2p: parse server state %s: %w", path, err)
	}
	state.normalize()
	return state, nil
}

func (s serverAppliedState) matches(reverse serverReverseState, redirects []redirect.Rule, forwards []forward.Rule, tunEnabled bool, tunName string, tunMTU int, tunAddr string) bool {
	s.normalize()
	reverse = normalizeReverse(reverse)
	return s.TunEnabled == tunEnabled &&
		strings.EqualFold(strings.TrimSpace(s.TunName), strings.TrimSpace(tunName)) &&
		s.TunMTU == tunMTU &&
		strings.EqualFold(strings.TrimSpace(s.TunAddr), strings.TrimSpace(tunAddr)) &&
		reflect.DeepEqual(s.Reverse, reverse) &&
		reflect.DeepEqual(s.Redirects, redirects) &&
		reflect.DeepEqual(s.Forwards, forwards)
}

func saveServerAppliedState(path string, reverse serverReverseState, redirects []redirect.Rule, forwards []forward.Rule, tunEnabled bool, tunName string, tunMTU int, tunAddr string) error {
	reverse = normalizeReverse(reverse)
	state := serverAppliedState{
		Reverse:    reverse,
		Redirects:  redirects,
		Forwards:   forwards,
		TunEnabled: tunEnabled,
		TunName:    strings.TrimSpace(tunName),
		TunMTU:     tunMTU,
		TunAddr:    strings.TrimSpace(tunAddr),
		Mode:       modeLabel(tunEnabled),
		Version:    version.Current(),
		Timestamp:  time.Now().UTC(),
	}
	data, err := json.MarshalIndent(state, "", "  ")
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

func (s *serverAppliedState) normalize() {
	s.Reverse = normalizeReverse(s.Reverse)
	if s.Redirects == nil {
		s.Redirects = []redirect.Rule{}
	}
	if s.Forwards == nil {
		s.Forwards = []forward.Rule{}
	}
	s.TunName = strings.TrimSpace(s.TunName)
	s.TunAddr = strings.TrimSpace(s.TunAddr)
}

func normalizeReverse(state serverReverseState) serverReverseState {
	if state == nil {
		return make(serverReverseState)
	}
	return state
}

func modeLabel(tunEnabled bool) string {
	if tunEnabled {
		return "tun"
	}
	return "proxy"
}
