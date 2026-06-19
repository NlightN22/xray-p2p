//go:build windows || linux

package server

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
)

// ForwardAddOptions describes server forward creation.
type ForwardAddOptions struct {
	InstallDir    string
	ConfigDir     string
	Target        string
	ListenAddress string
	ListenPort    int
	Protocol      forward.Protocol
	BasePort      int
}

// ForwardAddResult captures the applied forward alongside routing status.
type ForwardAddResult struct {
	Rule   forward.Rule
	Routed bool
}

// ForwardRemoveOptions controls server forward removal.
type ForwardRemoveOptions struct {
	InstallDir string
	ConfigDir  string
	Selector   forward.Selector
}

// ForwardListOptions configures forward enumeration.
type ForwardListOptions struct {
	InstallDir string
	ConfigDir  string
	Pending    bool
}

// AddForward registers a dokodemo-door forward in server configuration.
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

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return ForwardAddResult{}, err
	}

	store, err := openServerForwardStorePending()
	if err != nil {
		return ForwardAddResult{}, err
	}

	reserved := make(map[int]struct{}, len(store.forwards))
	for _, rule := range store.forwards {
		reserved[rule.ListenPort] = struct{}{}
	}

	listenPort := opts.ListenPort
	if listenPort > 0 {
		if err := forward.CheckPort(listenAddr, listenPort, proto); err != nil {
			if errors.Is(err, forward.ErrPortUnavailable) {
				return ForwardAddResult{}, fmt.Errorf("listen port %d is already in use on %s", listenPort, listenAddr)
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
	if err := store.add(rule); err != nil {
		return ForwardAddResult{}, err
	}
	store.doc[serverForwardRulesKey] = store.forwards

	var targetAddr netip.Addr
	if parsed, err := netip.ParseAddr(strings.TrimSpace(targetHost)); err == nil {
		targetAddr = parsed
	}

	if err := commitServerRuntimeDoc(context.Background(), store.doc); err != nil {
		return ForwardAddResult{}, err
	}
	_ = installDir
	return ForwardAddResult{
		Rule:   rule,
		Routed: forward.MatchesRedirect(store.redirects, targetAddr),
	}, nil
}

// RemoveForward deletes a server forward rule.
func RemoveForward(opts ForwardRemoveOptions) (forward.Rule, error) {
	if opts.Selector.Empty() {
		return forward.Rule{}, errors.New("--listen-port, --tag, or --remark is required")
	}

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return forward.Rule{}, err
	}

	store, err := openServerForwardStorePending()
	if err != nil {
		return forward.Rule{}, err
	}

	rule, idx, removed := store.remove(opts.Selector)
	if !removed {
		return forward.Rule{}, fmt.Errorf("forward rule not found")
	}

	store.doc[serverForwardRulesKey] = store.forwards
	if err := commitServerRuntimeDoc(context.Background(), store.doc); err != nil {
		store.insertAt(rule, idx)
		return forward.Rule{}, err
	}
	_ = installDir
	return rule, nil
}

// ListForwards returns configured server forwards.
func ListForwards(opts ForwardListOptions) ([]forward.Rule, error) {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return nil, err
	}

	var store serverForwardStore
	if opts.Pending {
		store, err = openServerForwardStoreFromPath(pendingConfigPath())
	} else {
		store, err = openServerForwardStore(installDir)
	}
	if err != nil {
		return nil, err
	}

	result := make([]forward.Rule, len(store.forwards))
	copy(result, store.forwards)
	return result, nil
}
