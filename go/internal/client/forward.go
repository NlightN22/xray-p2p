package client

import (
	"errors"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
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
}

// ForwardListOptions configures forward enumeration.
type ForwardListOptions struct {
	InstallDir string
	ConfigDir  string
}

// AddForward registers a dokodemo-door forward on the client.
func AddForward(opts ForwardAddOptions) (ForwardAddResult, error) {
	targetAddr, targetPort, err := forward.ParseTarget(opts.Target)
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

	paths, err := resolveClientPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return ForwardAddResult{}, err
	}

	state, err := loadClientInstallState(paths.stateFile)
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
		TargetIP:      targetAddr.String(),
		TargetPort:    targetPort,
		Protocol:      proto,
		Tag:           forward.TagForPort(listenPort),
		Remark:        forward.BuildRemark(targetAddr.String(), targetPort),
	}

	if err := state.addForward(rule); err != nil {
		return ForwardAddResult{}, err
	}
	if err := addClientForwardInbound(paths.configDir, rule); err != nil {
		return ForwardAddResult{}, err
	}
	if err := state.save(paths.stateFile); err != nil {
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

	paths, err := resolveClientPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return forward.Rule{}, err
	}

	state, err := loadClientInstallState(paths.stateFile)
	if err != nil {
		return forward.Rule{}, err
	}

	rule, idx, removed := state.removeForward(opts.Selector)
	if !removed {
		return forward.Rule{}, fmt.Errorf("xp2p: forward rule not found")
	}

	if err := removeClientForwardInbound(paths.configDir, rule); err != nil {
		state.insertForwardAt(rule, idx)
		return forward.Rule{}, err
	}
	if err := state.save(paths.stateFile); err != nil {
		state.insertForwardAt(rule, idx)
		return forward.Rule{}, err
	}
	return rule, nil
}

// ListForwards reports all configured forwards.
func ListForwards(opts ForwardListOptions) ([]forward.Rule, error) {
	paths, err := resolveClientPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return nil, err
	}

	state, err := loadClientInstallState(paths.stateFile)
	if err != nil {
		return nil, err
	}
	result := make([]forward.Rule, len(state.Forwards))
	copy(result, state.Forwards)
	return result, nil
}
