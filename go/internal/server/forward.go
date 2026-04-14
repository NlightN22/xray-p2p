//go:build windows || linux

package server

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
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
	desiredConfigDir, err := resolveUserConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return ForwardAddResult{}, err
	}
	liveConfigDir, err := config.LiveConfigDir(desiredConfigDir)
	if err != nil {
		return ForwardAddResult{}, err
	}
	configDir, err := pendingConfigDir(desiredConfigDir)
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
	if err := store.add(rule); err != nil {
		return ForwardAddResult{}, err
	}
	if err := store.saveForwards(); err != nil {
		return ForwardAddResult{}, err
	}
	xrayCfg, err := ensureServerXrayConfig(pendingConfigPath())
	if err != nil {
		return ForwardAddResult{}, err
	}
	cfg, err := loadServerConfigWithFallback()
	if err != nil {
		return ForwardAddResult{}, err
	}
	tunEnabled, tunName, tunMTU := cfg.Server.TunEnabled, cfg.Server.TunName, cfg.Server.TunMTU
	certPath := filepath.Join(liveConfigDir, "cert.pem")
	keyPath := filepath.Join(liveConfigDir, "key.pem")
	if strings.TrimSpace(cfg.Server.CertificateFile) != "" {
		certPath = cfg.Server.CertificateFile
	}
	if strings.TrimSpace(cfg.Server.KeyFile) != "" {
		keyPath = cfg.Server.KeyFile
	}
	clients, allowInsecure, err := resolvePendingTrojanClients(liveConfigDir, configDir, xrayCfg.Inbounds.Trojan.AllowInsecure)
	if err != nil {
		return ForwardAddResult{}, err
	}
	if err := writeServerInboundsConfigWithClients(configDir, xrayCfg, tunEnabled, tunName, tunMTU, parsePortOrDefault(cfg.Server.TrojanPort, DefaultTrojanPort), certPath, keyPath, allowInsecure, store.forwards, clients); err != nil {
		return ForwardAddResult{}, err
	}

	var targetAddr netip.Addr
	if parsed, err := netip.ParseAddr(strings.TrimSpace(targetHost)); err == nil {
		targetAddr = parsed
	}

	if err := writeServerApplyRequest(); err != nil {
		return ForwardAddResult{}, err
	}
	return ForwardAddResult{
		Rule:   rule,
		Routed: forward.MatchesRedirect(store.redirects, targetAddr),
	}, nil
}

// RemoveForward deletes a server forward rule.
func RemoveForward(opts ForwardRemoveOptions) (forward.Rule, error) {
	if opts.Selector.Empty() {
		return forward.Rule{}, errors.New("xp2p: --listen-port, --tag, or --remark is required")
	}

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return forward.Rule{}, err
	}

	desiredConfigDir, err := resolveUserConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return forward.Rule{}, err
	}
	liveConfigDir, err := config.LiveConfigDir(desiredConfigDir)
	if err != nil {
		return forward.Rule{}, err
	}
	configDir, err := pendingConfigDir(desiredConfigDir)
	if err != nil {
		return forward.Rule{}, err
	}

	store, err := openServerForwardStorePending()
	if err != nil {
		return forward.Rule{}, err
	}

	rule, idx, removed := store.remove(opts.Selector)
	if !removed {
		return forward.Rule{}, fmt.Errorf("xp2p: forward rule not found")
	}

	if err := store.saveForwards(); err != nil {
		store.insertAt(rule, idx)
		return forward.Rule{}, err
	}
	xrayCfg, err := ensureServerXrayConfig(pendingConfigPath())
	if err != nil {
		store.insertAt(rule, idx)
		return forward.Rule{}, err
	}
	cfg, err := loadServerConfigWithFallback()
	if err != nil {
		store.insertAt(rule, idx)
		return forward.Rule{}, err
	}
	tunEnabled, tunName, tunMTU := cfg.Server.TunEnabled, cfg.Server.TunName, cfg.Server.TunMTU
	certPath := filepath.Join(liveConfigDir, "cert.pem")
	keyPath := filepath.Join(liveConfigDir, "key.pem")
	if strings.TrimSpace(cfg.Server.CertificateFile) != "" {
		certPath = cfg.Server.CertificateFile
	}
	if strings.TrimSpace(cfg.Server.KeyFile) != "" {
		keyPath = cfg.Server.KeyFile
	}
	clients, allowInsecure, err := resolvePendingTrojanClients(liveConfigDir, configDir, xrayCfg.Inbounds.Trojan.AllowInsecure)
	if err != nil {
		store.insertAt(rule, idx)
		return forward.Rule{}, err
	}
	if err := writeServerInboundsConfigWithClients(configDir, xrayCfg, tunEnabled, tunName, tunMTU, parsePortOrDefault(cfg.Server.TrojanPort, DefaultTrojanPort), certPath, keyPath, allowInsecure, store.forwards, clients); err != nil {
		store.insertAt(rule, idx)
		return forward.Rule{}, err
	}
	if err := writeServerApplyRequest(); err != nil {
		store.insertAt(rule, idx)
		return forward.Rule{}, err
	}
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

func resolvePendingTrojanClients(liveConfigDir, pendingDir string, allowInsecure bool) ([]trojanClient, bool, error) {
	pendingInbounds := filepath.Join(pendingDir, "inbounds.json")
	if info, err := os.Stat(pendingInbounds); err == nil {
		if info.IsDir() {
			return nil, allowInsecure, fmt.Errorf("xp2p: %s is a directory, expected pending inbounds", pendingInbounds)
		}
		state, err := loadTrojanState(pendingDir)
		if err != nil {
			return nil, allowInsecure, err
		}
		if state.allowInsecure {
			allowInsecure = true
		}
		return state.clients, allowInsecure, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, allowInsecure, fmt.Errorf("xp2p: inspect %s: %w", pendingInbounds, err)
	}

	state, err := loadTrojanState(liveConfigDir)
	if err != nil {
		return nil, allowInsecure, err
	}
	if state.allowInsecure {
		allowInsecure = true
	}
	return state.clients, allowInsecure, nil
}
