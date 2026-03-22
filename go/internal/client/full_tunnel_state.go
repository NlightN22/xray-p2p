package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type fullTunnelState struct {
	Enabled      bool                             `json:"enabled"`
	TunName      string                           `json:"tun_name,omitempty"`
	TunMode      string                           `json:"tun_mode,omitempty"`
	IPv4Defaults []string                         `json:"ipv4_defaults,omitempty"`
	IPv6Defaults []string                         `json:"ipv6_defaults,omitempty"`
	BypassRoutes []fullTunnelRoute                `json:"bypass_routes,omitempty"`
	DNSBackup    *fullTunnelDNSBackup             `json:"dns_backup,omitempty"`
	EndpointIPs  map[string]fullTunnelEndpointIPs `json:"endpoint_ips,omitempty"`
	Timestamp    time.Time                        `json:"timestamp"`
}

type fullTunnelRoute struct {
	Family         string `json:"family,omitempty"`
	Route          string `json:"route,omitempty"`
	Destination    string `json:"destination,omitempty"`
	NextHop        string `json:"next_hop,omitempty"`
	InterfaceIndex int    `json:"interface_index,omitempty"`
	RouteMetric    int    `json:"route_metric,omitempty"`
	PolicyStore    string `json:"policy_store,omitempty"`
}

type fullTunnelDNSBackup struct {
	ResolvConf  string   `json:"resolv_conf,omitempty"`
	ResolvPath  string   `json:"resolv_path,omitempty"`
	WindowsIPv4 []string `json:"windows_ipv4,omitempty"`
	WindowsIPv6 []string `json:"windows_ipv6,omitempty"`
}

type fullTunnelEndpointIPs struct {
	IPv4       []string  `json:"ipv4,omitempty"`
	IPv6       []string  `json:"ipv6,omitempty"`
	ResolvedAt time.Time `json:"resolved_at,omitempty"`
}

func loadFullTunnelState(path string) (fullTunnelState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fullTunnelState{}, nil
		}
		return fullTunnelState{}, fmt.Errorf("xp2p: read full tunnel state %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return fullTunnelState{}, nil
	}
	var state fullTunnelState
	if err := json.Unmarshal(data, &state); err != nil {
		return fullTunnelState{}, fmt.Errorf("xp2p: parse full tunnel state %s: %w", path, err)
	}
	return state, nil
}

func saveFullTunnelState(path string, state fullTunnelState) error {
	state.Timestamp = time.Now().UTC()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("xp2p: encode full tunnel state %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure full tunnel state dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("xp2p: write full tunnel state %s: %w", path, err)
	}
	return nil
}

func clearFullTunnelState(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: remove full tunnel state %s: %w", path, err)
	}
	return nil
}
