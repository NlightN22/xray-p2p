package client

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

	"github.com/NlightN22/xray-p2p/go/internal/version"
)

type clientAppliedState struct {
	Config     clientInstallState `json:"config"`
	TunEnabled bool               `json:"tun_enabled"`
	TunName    string             `json:"tun_name"`
	TunMTU     int                `json:"tun_mtu"`
	TunAddr    string             `json:"tun_addr"`
	Mode       string             `json:"mode"`
	Runtime    clientRuntimeState `json:"runtime,omitempty"`
	Version    string             `json:"version"`
	Timestamp  time.Time          `json:"timestamp"`
}

type clientRuntimeState struct {
	Tun       tunRuntimeState   `json:"tun,omitempty"`
	Routes    routeRuntimeState `json:"routes,omitempty"`
	SocksReady bool             `json:"socks_ready,omitempty"`
	LastError string            `json:"last_error,omitempty"`
	Timestamp time.Time         `json:"timestamp,omitempty"`
}

type tunRuntimeState struct {
	Name       string `json:"name,omitempty"`
	IfIndex    int    `json:"if_index,omitempty"`
	IPv4       string `json:"ipv4,omitempty"`
	Prefix     int    `json:"prefix,omitempty"`
	OperStatus string `json:"oper_status,omitempty"`
	DadState   string `json:"dad_state,omitempty"`
	Ready      bool   `json:"ready,omitempty"`
	LastError  string `json:"last_error,omitempty"`
}

type routeRuntimeState struct {
	RedirectApplied  bool `json:"redirect_applied,omitempty"`
	RedirectCount    int  `json:"redirect_count,omitempty"`
	FullApplied      bool `json:"full_applied,omitempty"`
	FullBypassCount  int  `json:"full_bypass_count,omitempty"`
}

func loadClientAppliedState(path string) (clientAppliedState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return clientAppliedState{}, nil
		}
		return clientAppliedState{}, fmt.Errorf("xp2p: read client state %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return clientAppliedState{}, nil
	}
	var state clientAppliedState
	if err := json.Unmarshal(data, &state); err != nil {
		return clientAppliedState{}, fmt.Errorf("xp2p: parse client state %s: %w", path, err)
	}
	state.Config.normalize()
	state.TunName = strings.TrimSpace(state.TunName)
	state.TunAddr = strings.TrimSpace(state.TunAddr)
	return state, nil
}

func (s clientAppliedState) matches(cfg clientInstallState, tunEnabled bool, tunName string, tunMTU int, tunAddr string) bool {
	cfg.normalize()
	s.Config.normalize()
	return s.TunEnabled == tunEnabled &&
		strings.EqualFold(strings.TrimSpace(s.TunName), strings.TrimSpace(tunName)) &&
		s.TunMTU == tunMTU &&
		strings.EqualFold(strings.TrimSpace(s.TunAddr), strings.TrimSpace(tunAddr)) &&
		reflect.DeepEqual(s.Config, cfg)
}

func saveClientAppliedState(path string, cfg clientInstallState, tunEnabled bool, tunName string, tunMTU int, tunAddr string) error {
	cfg.normalize()
	state := clientAppliedState{
		Config:     cfg,
		TunEnabled: tunEnabled,
		TunName:    strings.TrimSpace(tunName),
		TunMTU:     tunMTU,
		TunAddr:    strings.TrimSpace(tunAddr),
		Mode:       modeLabel(tunEnabled),
		Version:    version.Current(),
		Timestamp:  time.Now().UTC(),
	}
	return writeClientAppliedState(path, state)
}

func modeLabel(tunEnabled bool) string {
	if tunEnabled {
		return "tun"
	}
	return "proxy"
}

func updateClientRuntimeState(path string, runtime clientRuntimeState) error {
	state, err := loadClientAppliedState(path)
	if err != nil {
		return err
	}
	runtime.Timestamp = time.Now().UTC()
	state.Runtime = runtime
	return writeClientAppliedState(path, state)
}

func writeClientAppliedState(path string, state clientAppliedState) error {
	state.Config.normalize()
	state.TunName = strings.TrimSpace(state.TunName)
	state.TunAddr = strings.TrimSpace(state.TunAddr)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("xp2p: encode client state %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure client state dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("xp2p: write client state %s: %w", path, err)
	}
	return nil
}
