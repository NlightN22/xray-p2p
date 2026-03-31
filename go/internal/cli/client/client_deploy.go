package clientcmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
	"github.com/NlightN22/xray-p2p/go/internal/health"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

const (
	deployLinkTTL                 = 10 * time.Minute
	socksReadyTimeout             = 30 * time.Second
	socksProbeInterval            = 500 * time.Millisecond
	deployCompletionNotifyTimeout = 30 * time.Second
	socksPingTimeout              = 45 * time.Second
	applyRequestTimeout           = 45 * time.Second
)

type manifestOptions struct {
	remoteHost     string
	installDir     string
	installDirSet  bool
	trojanPort     string
	trojanUser     string
	trojanPassword string
	tunMode        string
	tunModeSet     bool
	force          bool
}

type runtimeOptions struct {
	remoteHost string
	deployPort string
	serverHost string
	ciphertext []byte
}

type deployOptions struct {
	manifest manifestOptions
	runtime  runtimeOptions
}

func runClientDeploy(ctx context.Context, cfg config.Config, args []string) int {
	opts, err := parseDeployFlags(cfg, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		logging.Error("xp2p client deploy: argument parsing failed", "err", err)
		return 2
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
		logging.Error("xp2p client deploy: missing trojan link from server")
		return 1
	}

	logging.Info("xp2p client deploy: installing local client from trojan link")
	tl, err := parseTrojanLink(res.Link)
	if err != nil {
		logging.Error("xp2p client deploy: invalid trojan link", "err", err)
		return 1
	}

	installOpts := buildInstallOptionsFromLink(cfg, tl)
	installed, err := clishared.InstallPresent(clishared.InstallRoleClient, installOpts.InstallDir, installOpts.ConfigDir)
	if err != nil {
		completionState = "FAIL client-install-check"
		logging.Error("xp2p client deploy: install check failed", "err", err)
		return 1
	}
	if installed {
		logging.Info("xp2p client deploy: installation detected, appending endpoint", "install_dir", installOpts.InstallDir, "config_dir", installOpts.ConfigDir)
		if err := clientAddEndpointFunc(ctx, installOpts); err != nil {
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

	modeCfg, err := loadDeployClientConfig()
	if err != nil {
		completionState = "FAIL client-config"
		logging.Error("xp2p client deploy: load config failed", "err", err)
		return 1
	}

	tunMode := strings.TrimSpace(modeCfg.Client.TunMode)
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

	fullTunnelTag := strings.TrimSpace(modeCfg.Client.FullTunnelTag)
	if opts.manifest.tunModeSet && tunMode == "full" {
		tag, tagErr := resolveDeployFullTunnelTag(installOpts.InstallDir, installOpts.ConfigDir, tl, opts.runtime)
		if tagErr != nil {
			logging.Warn("xp2p client deploy: full-tunnel tag resolution failed", "err", tagErr)
		} else if strings.TrimSpace(tag) != "" {
			fullTunnelTag = tag
		}
	}

	if err := applyClientDeployMode(installOpts, cfg, false, tunMode, opts.manifest.tunModeSet, fullTunnelTag); err != nil {
		completionState = "FAIL client-mode-proxy"
		logging.Error("xp2p client deploy: proxy mode setup failed", "err", err)
		return 1
	}

	cancelDiagnostics := startDiagnostics(ctx, cfg.Client.DiagPort)
	if cancelDiagnostics != nil {
		defer cancelDiagnostics()
	}

	socksAddr := strings.TrimSpace(cfg.Client.SocksAddress)
	if err := ensureClientServiceApplied(ctx, socksAddr); err != nil {
		completionState = "FAIL service-apply"
		logging.Error("xp2p client deploy: service apply failed", "err", err)
		return 1
	}
	if socksAddr != "" {
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

	if err := applyClientDeployMode(installOpts, cfg, true, tunMode, opts.manifest.tunModeSet, fullTunnelTag); err != nil {
		completionState = "FAIL client-mode-tun"
		logging.Error("xp2p client deploy: tun mode setup failed", "err", err)
		return 1
	}
	if err := ensureClientServiceApplied(ctx, socksAddr); err != nil {
		completionState = "FAIL service-apply"
		logging.Error("xp2p client deploy: service apply failed", "err", err)
		return 1
	}

	completionState = "OK"
	logging.Info("xp2p client deploy: completed")
	return 0
}

func parseDeployFlags(cfg config.Config, args []string) (deployOptions, error) {
	fs := flag.NewFlagSet("xp2p client deploy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	hostFlag := fs.String("host", "", "deploy host name or address")
	deployPort := fs.String("port", "62025", "deploy port")
	installDir := fs.String("install-dir", "", "server install directory override")
	trojanUser := fs.String("user", "", "Trojan user identifier (email)")
	trojanPassword := fs.String("password", "", "Trojan user password (auto-generated when omitted)")
	trojanPort := fs.String("trojan-port", "", "Trojan service port")
	tunMode := fs.String("tun-mode", "", "TUN routing mode (split or full)")
	force := fs.Bool("force", false, "allow changing existing tun mode")

	if err := fs.Parse(args); err != nil {
		return deployOptions{}, err
	}
	if fs.NArg() > 0 {
		return deployOptions{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	host := strings.TrimSpace(*hostFlag)
	if host == "" || strings.HasPrefix(host, "-") {
		return deployOptions{}, fmt.Errorf("--host is required")
	}
	if err := netutil.ValidateHost(host); err != nil {
		return deployOptions{}, fmt.Errorf("--host: %v", err)
	}

	serverHostValue := firstNonEmpty(cfg.Server.Host, host)
	serverPortValue := normalizeServerPort(cfg, *trojanPort)

	userValue := strings.TrimSpace(firstNonEmpty(*trojanUser, cfg.Client.User))
	if userValue == "" {
		userValue = fmt.Sprintf("deploy-%d@local", time.Now().Unix())
	}

	passwordValue := strings.TrimSpace(*trojanPassword)
	if passwordValue == "" {
		passwordValue = strings.TrimSpace(cfg.Client.Password)
	}
	if passwordValue == "" && userValue != "" {
		gen, err := generateSecret(18)
		if err != nil {
			return deployOptions{}, fmt.Errorf("generate password: %w", err)
		}
		passwordValue = gen
	}
	if err := clishared.ValidateRFC3986Unreserved(passwordValue); err != nil {
		return deployOptions{}, fmt.Errorf("invalid password: %w", err)
	}

	installDirSet := false
	tunModeSet := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == "install-dir" {
			installDirSet = true
		}
		if flag.Name == "tun-mode" {
			tunModeSet = true
		}
	})

	tunModeValue := strings.TrimSpace(*tunMode)
	if tunModeSet {
		switch strings.ToLower(tunModeValue) {
		case "split", "full":
			tunModeValue = strings.ToLower(tunModeValue)
		default:
			return deployOptions{}, fmt.Errorf("--tun-mode must be split or full")
		}
	}

	return deployOptions{
		manifest: manifestOptions{
			remoteHost:     host,
			installDir:     strings.TrimSpace(*installDir),
			installDirSet:  installDirSet,
			trojanPort:     serverPortValue,
			trojanUser:     strings.TrimSpace(userValue),
			trojanPassword: strings.TrimSpace(passwordValue),
			tunMode:        tunModeValue,
			tunModeSet:     tunModeSet,
			force:          *force,
		},
		runtime: runtimeOptions{
			remoteHost: host,
			deployPort: strings.TrimSpace(*deployPort),
			serverHost: serverHostValue,
		},
	}, nil
}

func buildDeployLink(opts *deployOptions) (string, error) {
	installDir := ""
	if opts.manifest.installDirSet {
		installDir = strings.TrimSpace(opts.manifest.installDir)
	}
	manifest := spec.Manifest{
		Host:           strings.TrimSpace(opts.runtime.serverHost),
		Version:        2,
		InstallDir:     installDir,
		TrojanPort:     strings.TrimSpace(opts.manifest.trojanPort),
		TrojanUser:     strings.TrimSpace(opts.manifest.trojanUser),
		TrojanPassword: strings.TrimSpace(opts.manifest.trojanPassword),
	}
	linkURL, enc, err := deploylink.Build(opts.runtime.remoteHost, opts.runtime.deployPort, manifest, deployLinkTTL)
	if err != nil {
		return "", err
	}
	opts.runtime.ciphertext = enc.Ciphertext
	return linkURL, nil
}

func ensureDeployTargetAvailable(cfg config.Config, opts deployOptions) error {
	host := strings.TrimSpace(opts.runtime.serverHost)
	if host == "" {
		host = strings.TrimSpace(opts.runtime.remoteHost)
	}
	if host == "" {
		return fmt.Errorf("xp2p: deploy host is required")
	}
	portStr := strings.TrimSpace(opts.manifest.trojanPort)
	if portStr == "" {
		return nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("xp2p: invalid trojan port %q", portStr)
	}

	records, err := clientListFunc(client.ListOptions{
		InstallDir: strings.TrimSpace(cfg.Client.InstallDir),
		ConfigDir:  strings.TrimSpace(cfg.Client.ConfigDir),
	})
	if err != nil {
		return err
	}
	for _, record := range records {
		if strings.EqualFold(record.Hostname, host) && record.Port == port {
			return fmt.Errorf("xp2p: endpoint %s:%d already exists", host, port)
		}
	}
	return nil
}

// buildInstallOptionsFromLink converts a parsed trojan link into client install options,
// applying config defaults for install paths.
func buildInstallOptionsFromLink(cfg config.Config, link trojanLink) client.InstallOptions {
	allowInsecure := link.AllowInsecure
	if link.PinnedPeerSHA256 != "" {
		allowInsecure = false
	}
	return client.InstallOptions{
		InstallDir:           cfg.Client.InstallDir,
		ConfigDir:            cfg.Client.ConfigDir,
		ServerAddress:        link.ServerAddress,
		ServerPort:           link.ServerPort,
		User:                 link.User,
		Password:             link.Password,
		ServerName:           link.ServerName,
		ALPN:                 link.ALPN,
		AllowInsecure:        allowInsecure,
		PinnedPeerCertSHA256: link.PinnedPeerSHA256,
		VerifyPeerCertByName: link.VerifyPeerName,
		Force:                true,
		TunEnabled:           cfg.Client.TunEnabled,
		TunEnabledSet:        true,
		TunName:              cfg.Client.TunName,
		TunMTU:               cfg.Client.TunMTU,
		TunAddr:              cfg.Client.TunAddr,
		TunMode:              cfg.Client.TunMode,
	}
}

func waitForPing(ctx context.Context, host string, opts ping.Options, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := ping.Run(ctx, host, opts); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return fmt.Errorf("ping timeout after %s", timeout)
		}
		time.Sleep(1 * time.Second)
	}
}

func applyClientDeployMode(installOpts client.InstallOptions, cfg config.Config, tunEnabled bool, tunMode string, tunModeSet bool, fullTunnelTag string) error {
	modeLabel := "proxy"
	if tunEnabled {
		modeLabel = "tun"
	}
	updatedPath, err := config.UpdateTunEnabled("", "client", tunEnabled)
	if err != nil {
		return err
	}
	if _, err := config.EnsureTunSettings("", "client", tunEnabled, cfg.Client.TunName, cfg.Client.TunMTU, cfg.Client.TunAddr); err != nil {
		return err
	}
	if tunModeSet {
		if _, err := config.UpdateTunMode("", "client", tunMode); err != nil {
			return err
		}
	}
	if tunModeSet && strings.EqualFold(strings.TrimSpace(tunMode), "full") && strings.TrimSpace(fullTunnelTag) != "" {
		if _, err := config.UpdateFullTunnelTag("", fullTunnelTag); err != nil {
			return err
		}
	}
	logging.Info("xp2p client deploy: mode config updated", "mode", modeLabel, "config", updatedPath)
	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
}

func loadDeployClientConfig() (config.Config, error) {
	path := config.ConfigPath(layout.ClientConfigFileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			path = config.PendingConfigPath(layout.ClientConfigFileName)
		} else {
			return config.Config{}, err
		}
	}
	return config.Load(config.Options{
		Path:         path,
		AllowInvalid: true,
	})
}

func resolveDeployFullTunnelTag(installDir, configDir string, link trojanLink, runtime runtimeOptions) (string, error) {
	host := strings.TrimSpace(link.ServerAddress)
	if host == "" {
		host = strings.TrimSpace(runtime.serverHost)
	}
	if host == "" {
		host = strings.TrimSpace(runtime.remoteHost)
	}
	if host == "" {
		return "", fmt.Errorf("xp2p: deploy host is required for full-tunnel")
	}

	records, err := clientListFunc(client.ListOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		Pending:    !clientLiveConfigPresent(),
	})
	if err != nil {
		return "", err
	}
	for _, record := range records {
		if strings.EqualFold(record.Hostname, host) && strings.TrimSpace(record.Tag) != "" {
			return record.Tag, nil
		}
	}
	return "", fmt.Errorf("xp2p: full-tunnel endpoint %s not found", host)
}

func ensureClientServiceApplied(ctx context.Context, socksAddr string) error {
	ctrl := servicecontrol.Default()
	status, err := ctrl.Status(ctx, servicecontrol.RoleClient)
	if err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			return err
		}
		return err
	}
	if !status.Active {
		if err := clishared.RequireRoot(); err != nil {
			return err
		}
		if err := ctrl.Start(ctx, servicecontrol.RoleClient); err != nil {
			return err
		}
	}
	if err := waitForApplyRequestClear(ctx, config.ApplyRequestPath(), applyRequestTimeout); err != nil {
		return err
	}
	if strings.TrimSpace(socksAddr) == "" {
		return nil
	}
	return health.WaitForSocksProxy(ctx, socksAddr, socksReadyTimeout, socksProbeInterval)
}

func clientLiveConfigPresent() bool {
	path := config.ConfigPath(layout.ClientConfigFileName)
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func waitForApplyRequestClear(ctx context.Context, path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("apply request still present after %s", timeout)
}
