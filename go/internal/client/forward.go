//go:build linux || windows

package client

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

// ForwardAddOptions controls client forward creation.
type ForwardAddOptions struct {
	InstallDir    string
	ConfigDir     string
	Target        string
	ListenAddress string
	ListenPort    int
	Protocol      forward.Protocol
	BasePort      int
}

// ForwardAddResult describes the newly added rule.
type ForwardAddResult struct {
	Rule   forward.Rule
	Routed bool
}

// ForwardRemoveOptions controls forward deletion.
type ForwardRemoveOptions struct {
	InstallDir string
	ConfigDir  string
	Selector   forward.Selector
	Cleanup    bool
}

// ForwardListOptions configures forward enumeration.
type ForwardListOptions struct {
	InstallDir string
	ConfigDir  string
	Pending    bool
}

// AddForward registers a dokodemo-door forward on the client.
func AddForward(opts ForwardAddOptions) (ForwardAddResult, error) {
	targetHost, targetPort, err := forward.ParseTarget(opts.Target)
	if err != nil {
		return ForwardAddResult{}, err
	}
	listenAddr, err := forward.NormalizeListenAddress(opts.ListenAddress)
	if err != nil {
		return ForwardAddResult{}, err
	}

	proto := opts.Protocol
	if proto == "" {
		proto = forward.ProtocolBoth
	}

	configFile := config.ConfigPath(layout.ClientConfigFileName)

	state, err := loadClientInstallState(configFile)
	if err != nil {
		return ForwardAddResult{}, err
	}

	reserved := make(map[int]struct{}, len(state.Forwards))
	for _, rule := range state.Forwards {
		reserved[rule.ListenPort] = struct{}{}
	}

	listenPort := opts.ListenPort
	if listenPort > 0 {
		if err := forward.CheckPort(listenAddr, listenPort, proto); err != nil {
			if errors.Is(err, forward.ErrPortUnavailable) {
				return ForwardAddResult{}, fmt.Errorf("xp2p: listen port %d is already in use on %s", listenPort, listenAddr)
			}
			return ForwardAddResult{}, err
		}
	} else {
		base := opts.BasePort
		if base <= 0 {
			base = forward.DefaultBasePort
		}
		listenPort, err = forward.FindAvailablePort(listenAddr, base, proto, reserved)
		if err != nil {
			return ForwardAddResult{}, err
		}
	}

	rule := forward.Rule{
		ListenAddress: listenAddr,
		ListenPort:    listenPort,
		TargetHost:    targetHost,
		TargetPort:    targetPort,
		Protocol:      proto,
		Tag:           forward.TagForPort(listenPort),
		Remark:        forward.BuildRemark(targetHost, targetPort),
	}

	if err := state.addForward(rule); err != nil {
		return ForwardAddResult{}, err
	}
	if err := state.save(configFile); err != nil {
		return ForwardAddResult{}, err
	}

	var targetAddr netip.Addr
	if parsed, err := netip.ParseAddr(strings.TrimSpace(targetHost)); err == nil {
		targetAddr = parsed
	}
	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return ForwardAddResult{}, err
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		return ForwardAddResult{}, err
	}
	return ForwardAddResult{
		Rule:   rule,
		Routed: forward.MatchesRedirect(state.Redirects, targetAddr),
	}, nil
}

// RemoveForward deletes a client forward rule.
func RemoveForward(opts ForwardRemoveOptions) (forward.Rule, error) {
	if opts.Selector.Empty() {
		return forward.Rule{}, errors.New("xp2p: --listen-port, --tag, or --remark is required")
	}

	configFile := config.ConfigPath(layout.ClientConfigFileName)

	state, err := loadClientInstallState(configFile)
	if err != nil {
		return forward.Rule{}, err
	}

	rule, idx, removed := state.removeForward(opts.Selector)
	if !removed {
		return forward.Rule{}, fmt.Errorf("xp2p: forward rule not found")
	}

	if err := state.save(configFile); err != nil {
		state.insertForwardAt(rule, idx)
		return forward.Rule{}, err
	}
	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		state.insertForwardAt(rule, idx)
		return forward.Rule{}, err
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		state.insertForwardAt(rule, idx)
		return forward.Rule{}, err
	}
	return rule, nil
}

// ListForwards reports all configured forwards.
func ListForwards(opts ForwardListOptions) ([]forward.Rule, error) {
	state, err := loadClientInstallState(config.ConfigPath(layout.ClientConfigFileName))
	if err != nil {
		return nil, err
	}
	result := make([]forward.Rule, len(state.Forwards))
	copy(result, state.Forwards)
	return result, nil
}
