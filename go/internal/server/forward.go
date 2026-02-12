//go:build windows || linux

package server

import (
	"errors"
	"fmt"
	"net/netip"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
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
	configDir, err := resolveUserConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return ForwardAddResult{}, err
	}

	store, err := openServerForwardStore(installDir)
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
	xrayCfg, err := ensureServerXrayConfig(filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)))
	if err != nil {
		return ForwardAddResult{}, err
	}
	tunEnabled, tunName, tunMTU, err := loadServerTunSettings(filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)))
	if err != nil {
		return ForwardAddResult{}, err
	}
	cfg, err := config.Load(config.Options{Path: config.ConfigPath(layout.ServerConfigFileName)})
	if err != nil {
		return ForwardAddResult{}, err
	}
	certPath := filepath.Join(configDir, "cert.pem")
	keyPath := filepath.Join(configDir, "key.pem")
	if strings.TrimSpace(cfg.Server.CertificateFile) != "" {
		certPath = cfg.Server.CertificateFile
	}
	if strings.TrimSpace(cfg.Server.KeyFile) != "" {
		keyPath = cfg.Server.KeyFile
	}
	if err := writeServerInboundsConfig(configDir, xrayCfg, tunEnabled, tunName, tunMTU, parsePortOrDefault(cfg.Server.TrojanPort, DefaultTrojanPort), certPath, keyPath, xrayCfg.Inbounds.Trojan.AllowInsecure, store.forwards); err != nil {
		return ForwardAddResult{}, err
	}

	var targetAddr netip.Addr
	if parsed, err := netip.ParseAddr(strings.TrimSpace(targetHost)); err == nil {
		targetAddr = parsed
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

	configDir, err := resolveUserConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return forward.Rule{}, err
	}

	store, err := openServerForwardStore(installDir)
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
	xrayCfg, err := ensureServerXrayConfig(filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)))
	if err != nil {
		store.insertAt(rule, idx)
		return forward.Rule{}, err
	}
	tunEnabled, tunName, tunMTU, err := loadServerTunSettings(filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)))
	if err != nil {
		store.insertAt(rule, idx)
		return forward.Rule{}, err
	}
	cfg, err := config.Load(config.Options{Path: config.ConfigPath(layout.ServerConfigFileName)})
	if err != nil {
		store.insertAt(rule, idx)
		return forward.Rule{}, err
	}
	certPath := filepath.Join(configDir, "cert.pem")
	keyPath := filepath.Join(configDir, "key.pem")
	if strings.TrimSpace(cfg.Server.CertificateFile) != "" {
		certPath = cfg.Server.CertificateFile
	}
	if strings.TrimSpace(cfg.Server.KeyFile) != "" {
		keyPath = cfg.Server.KeyFile
	}
	if err := writeServerInboundsConfig(configDir, xrayCfg, tunEnabled, tunName, tunMTU, parsePortOrDefault(cfg.Server.TrojanPort, DefaultTrojanPort), certPath, keyPath, xrayCfg.Inbounds.Trojan.AllowInsecure, store.forwards); err != nil {
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

	store, err := openServerForwardStore(installDir)
	if err != nil {
		return nil, err
	}

	result := make([]forward.Rule, len(store.forwards))
	copy(result, store.forwards)
	return result, nil
}
