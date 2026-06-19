package clientcmd

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"time"

	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/preflight"
)

func runClientDeploy(ctx context.Context, cfg config.Config, args []string) int {
	opts, err := parseDeployFlags(cfg, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		logging.Error("xp2p client deploy: argument parsing failed", "err", err)
		return 2
	}

	if opts.manifest.mode.set && opts.manifest.mode.tunEnabled {
		wintunPath := filepath.Join(cfg.Client.InstallDir, layout.BinDirName, "wintun.dll")
		if err := tunPreflightCheckFunc(ctx, preflight.TunConfig{
			Enabled:       true,
			Name:          cfg.Client.TunName,
			Addr:          cfg.Client.TunAddr,
			MTU:           cfg.Client.TunMTU,
			Mode:          cfg.Client.TunMode,
			WintunDLLPath: wintunPath,
		}); err != nil {
			logging.Error("xp2p client deploy: tun preflight failed", "err", err)
			return 1
		}
	}

	// Build and print deploy link (v2 encrypted), then run handshake
	if err := ensureDeployTargetAvailable(cfg, opts); err != nil {
		logging.Error("xp2p client deploy: endpoint already exists", "err", err)
		return 1
	}

	linkURL, err := buildDeployLink(&opts)
	if err != nil {
		logging.Error("xp2p client deploy: build link failed", "err", err)
		return 2
	}
	logging.Info("xp2p client deploy: link generated", "link", linkURL)
	logging.Info("xp2p client deploy: waiting for server...", "remote_host", opts.runtime.remoteHost, "deploy_port", opts.runtime.deployPort)

	// Retry handshake until server is ready or timeout elapses.
	var (
		res             deployResult
		handshakeErr    error
		notifyComplete  deployCompletionFunc
		completionState = "FAIL"
	)
	deadline := time.Now().Add(10 * time.Minute)
	backoff := 2 * time.Second
	if backoff <= 0 {
		backoff = 2 * time.Second
	}
	for {
		if ctx.Err() != nil {
			logging.Error("xp2p client deploy: cancelled", "err", ctx.Err())
			return 1
		}
		res, notifyComplete, handshakeErr = performDeployHandshake(ctx, opts)
		if handshakeErr == nil {
			break
		}
		if isServerDeployError(handshakeErr) {
			logging.Error("xp2p client deploy: server rejected deploy request", "err", handshakeErr)
			return 1
		}
		if time.Now().After(deadline) {
			logging.Error("xp2p client deploy: handshake timeout", "err", handshakeErr)
			return 1
		}
		logging.Debug("xp2p client deploy: server not ready, retrying", "next_in", backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			logging.Error("xp2p client deploy: cancelled", "err", ctx.Err())
			return 1
		}
		if backoff < 5*time.Second {
			backoff += 1 * time.Second
		}
	}
	if notifyComplete != nil {
		defer func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), deployCompletionNotifyTimeout)
			defer cancel()
			if err := notifyComplete(notifyCtx, completionState); err != nil {
				logging.Warn("xp2p client deploy: completion notify failed", "err", err)
			}
		}()
	}

	if res.ExitCode != 0 {
		completionState = "FAIL server-install"
		logging.Error("xp2p client deploy: server install failed", "exit_code", res.ExitCode)
		return 1
	}
	if strings.TrimSpace(res.Link) == "" {
		completionState = "FAIL server-link"
		logging.Error("xp2p client deploy: missing connection link from server")
		return 1
	}

	logging.Info("xp2p client deploy: installing local client from connection link")
	tl, err := parseTrojanLink(res.Link)
	if err != nil {
		logging.Error("xp2p client deploy: invalid connection link", "err", err)
		return 1
	}

	installOpts := buildInstallOptionsFromLink(cfg, tl)
	if opts.manifest.mode.set {
		installOpts.TunEnabled = opts.manifest.mode.tunEnabled
		installOpts.TunEnabledSet = true
		if installOpts.TunEnabled && opts.manifest.tunModeSet {
			installOpts.TunMode = opts.manifest.tunMode
			installOpts.TunModeSet = true
		}
		if !installOpts.TunEnabled {
			installOpts.TunModeSet = false
		}
	}
	installed, err := clishared.InstallPresent(clishared.InstallRoleClient, installOpts.InstallDir, installOpts.ConfigDir)
	if err != nil {
		completionState = "FAIL client-install-check"
		logging.Error("xp2p client deploy: install check failed", "err", err)
		return 1
	}
	if installed {
		logging.Info("xp2p client deploy: installation detected, appending endpoint", "install_dir", installOpts.InstallDir, "config_dir", installOpts.ConfigDir)
		if err := clientStageEndpointFunc(ctx, installOpts); err != nil {
			completionState = "FAIL client-append"
			logging.Error("xp2p client deploy: append endpoint failed", "err", err)
			return 1
		}
		logging.Info("xp2p client deploy: endpoint added", "install_dir", installOpts.InstallDir, "config_dir", installOpts.ConfigDir)
	} else {
		if err := clientInstallFunc(ctx, installOpts); err != nil {
			completionState = "FAIL client-install"
			logging.Error("xp2p client deploy: local install failed", "err", err)
			return 1
		}
		logging.Info("xp2p client deploy: local install completed", "install_dir", installOpts.InstallDir, "config_dir", installOpts.ConfigDir)
	}

	finalTunEnabled := true
	if opts.manifest.mode.set {
		finalTunEnabled = opts.manifest.mode.tunEnabled
	}

	tunMode := ""
	fullTunnelTag := ""
	if finalTunEnabled {
		modeCfg, err := loadDeployClientConfig()
		if err != nil {
			completionState = "FAIL client-config"
			logging.Error("xp2p client deploy: load config failed", "err", err)
			return 1
		}

		tunMode = strings.TrimSpace(modeCfg.Client.TunMode)
		if opts.manifest.tunModeSet {
			if installed && !strings.EqualFold(tunMode, opts.manifest.tunMode) && !opts.manifest.force {
				completionState = "FAIL client-mode-conflict"
				logging.Error("xp2p client deploy: tun mode conflict (use --force to override)", "current", tunMode, "requested", opts.manifest.tunMode)
				return 1
			}
			tunMode = opts.manifest.tunMode
			if installed && !strings.EqualFold(modeCfg.Client.TunMode, tunMode) {
				logging.Warn("xp2p client deploy: overriding existing tun mode", "from", modeCfg.Client.TunMode, "to", tunMode)
			}
		}

		fullTunnelTag = strings.TrimSpace(modeCfg.Client.FullTunnelTag)
		if opts.manifest.tunModeSet && tunMode == "full" {
			tag, tagErr := resolveDeployFullTunnelTag(installOpts.InstallDir, installOpts.ConfigDir, tl, opts.runtime)
			if tagErr != nil {
				logging.Warn("xp2p client deploy: full-tunnel tag resolution failed", "err", tagErr)
			} else if strings.TrimSpace(tag) != "" {
				fullTunnelTag = tag
			}
		}
	}

	if err := applyClientDeployMode(installOpts, cfg, false, tunMode, opts.manifest.tunModeSet, fullTunnelTag); err != nil {
		completionState = "FAIL client-mode-proxy"
		logging.Error("xp2p client deploy: proxy mode setup failed", "err", err)
		return 1
	}

	socksAddr := strings.TrimSpace(cfg.Client.SocksAddress)
	serviceActive, err := ensureClientServiceApplied(ctx, socksAddr)
	if err != nil {
		completionState = "FAIL service-apply"
		logging.Error("xp2p client deploy: service apply failed", "err", err)
		return 1
	}
	if !serviceActive {
		logging.Info("xp2p client deploy: service inactive; skipping SOCKS ping")
	} else if socksAddr != "" {
		targetHost := strings.TrimSpace(tl.ServerAddress)
		if targetHost == "" {
			targetHost = strings.TrimSpace(opts.runtime.serverHost)
		}
		if targetHost == "" {
			targetHost = strings.TrimSpace(opts.runtime.remoteHost)
		}

		markerTarget, markerPort, err := client.ResolveMarkerTarget(installOpts.InstallDir, targetHost, "", 0)
		if err != nil {
			completionState = "FAIL marker"
			logging.Error("xp2p client deploy: marker resolution failed", "err", err)
			return 1
		}

		logging.Info("xp2p client deploy: verifying connectivity via SOCKS ping", "target", targetHost, "marker", markerTarget)
		pingOpts := ping.Options{
			Count:      1,
			Timeout:    3 * time.Second,
			Proto:      "tcp",
			Port:       markerPort,
			SocksProxy: socksAddr,
		}
		if err := waitForPing(ctx, markerTarget, pingOpts, socksPingTimeout); err != nil {
			completionState = "FAIL ping"
			logging.Error("xp2p client deploy: ping failed", "err", err)
			return 1
		}
		logging.Info("xp2p client deploy: ping ok")
	} else {
		logging.Warn("xp2p client deploy: socks proxy address missing; skipping ping")
	}

	if finalTunEnabled {
		wintunPath := filepath.Join(installOpts.InstallDir, layout.BinDirName, "wintun.dll")
		if err := tunPreflightCheckFunc(ctx, preflight.TunConfig{
			Enabled:       true,
			Name:          installOpts.TunName,
			Addr:          installOpts.TunAddr,
			MTU:           installOpts.TunMTU,
			Mode:          installOpts.TunMode,
			WintunDLLPath: wintunPath,
		}); err != nil {
			completionState = "FAIL tun-preflight"
			logging.Error("xp2p client deploy: tun preflight failed", "err", err)
			return 1
		}
		if err := applyClientDeployMode(installOpts, cfg, true, tunMode, opts.manifest.tunModeSet, fullTunnelTag); err != nil {
			completionState = "FAIL client-mode-tun"
			logging.Error("xp2p client deploy: tun mode setup failed", "err", err)
			return 1
		}
		_, err = ensureClientServiceApplied(ctx, socksAddr)
		if err != nil {
			completionState = "FAIL service-apply"
			logging.Error("xp2p client deploy: service apply failed", "err", err)
			return 1
		}
	}

	completionState = "OK"
	logClientServiceApplyHint(ctx)
	logging.Info("xp2p client deploy: completed")
	return 0
}
