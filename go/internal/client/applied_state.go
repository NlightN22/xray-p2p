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
	"github.com/NlightN22/xray-p2p/go/internal/xrayguard"
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
	Status         string                 `json:"status,omitempty"`
	Reason         string                 `json:"reason,omitempty"`
	Tun            tunRuntimeState        `json:"tun,omitempty"`
	Routes         routeRuntimeState      `json:"routes,omitempty"`
	LoopProtection *loopProtectionRuntime `json:"loop_protection,omitempty"`
	SocksReady     bool                   `json:"socks_ready,omitempty"`
	LastError      string                 `json:"last_error,omitempty"`
	Timestamp      time.Time              `json:"timestamp,omitempty"`
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
	RedirectApplied bool `json:"redirect_applied,omitempty"`
	RedirectCount   int  `json:"redirect_count,omitempty"`
	FullApplied     bool `json:"full_applied,omitempty"`
	FullBypassCount int  `json:"full_bypass_count,omitempty"`
}

type loopProtectionRuntime struct {
	Reason              string    `json:"reason,omitempty"`
	Action              string    `json:"action,omitempty"`
	PID                 int       `json:"pid,omitempty"`
	FDBefore            int       `json:"fd_before,omitempty"`
	FDAfter             int       `json:"fd_after,omitempty"`
	FDDelta             int       `json:"fd_delta,omitempty"`
	Window              string    `json:"window,omitempty"`
	SocketRatioPercent  int       `json:"socket_ratio_percent,omitempty"`
	EstablishedTCPCount int       `json:"established_tcp,omitempty"`
	RelatedOutbound     string    `json:"related_outbound,omitempty"`
	DetectedAt          time.Time `json:"detected_at,omitempty"`
}

type RuntimeStatus struct {
	Status             string
	Reason             string
	LastError          string
	RelatedOutbound    string
	FDBefore           int
	FDAfter            int
	FDDelta            int
	Window             string
	Action             string
	DetectedAt         time.Time
	LoopProtectionSeen bool
}

func loadClientAppliedState(path string) (clientAppliedState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return clientAppliedState{}, nil
		}
		return clientAppliedState{}, fmt.Errorf("read client state %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return clientAppliedState{}, nil
	}
	var state clientAppliedState
	if err := json.Unmarshal(data, &state); err != nil {
		return clientAppliedState{}, fmt.Errorf("parse client state %s: %w", path, err)
	}
	state.Config.normalize()
	state.TunName = strings.TrimSpace(state.TunName)
	state.TunAddr = strings.TrimSpace(state.TunAddr)
	return state, nil
}

func LoadRuntimeStatus(path string) (RuntimeStatus, error) {
	state, err := loadClientAppliedState(path)
	if err != nil {
		return RuntimeStatus{}, err
	}
	runtime := state.Runtime
	status := RuntimeStatus{
		Status:    strings.TrimSpace(runtime.Status),
		Reason:    strings.TrimSpace(runtime.Reason),
		LastError: strings.TrimSpace(runtime.LastError),
	}
	if runtime.LoopProtection != nil {
		loop := runtime.LoopProtection
		status.RelatedOutbound = strings.TrimSpace(loop.RelatedOutbound)
		status.FDBefore = loop.FDBefore
		status.FDAfter = loop.FDAfter
		status.FDDelta = loop.FDDelta
		status.Window = strings.TrimSpace(loop.Window)
		status.Action = strings.TrimSpace(loop.Action)
		status.DetectedAt = loop.DetectedAt
		status.LoopProtectionSeen = true
	}
	return status, nil
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

func updateClientRuntimeQuarantine(path string, event xrayguard.Event, relatedOutbound string) error {
	return updateClientRuntimeState(path, clientRuntimeState{
		Status:    "quarantined",
		Reason:    event.Reason,
		LastError: event.Error(),
		LoopProtection: &loopProtectionRuntime{
			Reason:              event.Reason,
			Action:              event.Action,
			PID:                 event.PID,
			FDBefore:            event.Before.FDCount,
			FDAfter:             event.After.FDCount,
			FDDelta:             event.FDDelta,
			Window:              event.Window.String(),
			SocketRatioPercent:  event.SocketRatioPercent,
			EstablishedTCPCount: event.EstablishedTCPCount,
			RelatedOutbound:     strings.TrimSpace(relatedOutbound),
			DetectedAt:          time.Now().UTC(),
		},
	})
}

func writeClientAppliedState(path string, state clientAppliedState) error {
	state.Config.normalize()
	state.TunName = strings.TrimSpace(state.TunName)
	state.TunAddr = strings.TrimSpace(state.TunAddr)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode client state %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure client state dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write client state %s: %w", path, err)
	}
	return nil
}
