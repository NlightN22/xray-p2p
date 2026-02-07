package clientcmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
)

const (
	deployLinkTTL                 = 10 * time.Minute
	socksReadyTimeout             = 30 * time.Second
	socksProbeInterval            = 500 * time.Millisecond
	deployCompletionNotifyTimeout = 30 * time.Second
)

type manifestOptions struct {
	remoteHost     string
	installDir     string
	installDirSet  bool
	trojanPort     string
	trojanUser     string
	trojanPassword string
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
	if err := clientInstallFunc(ctx, installOpts); err != nil {
		completionState = "FAIL client-install"
		logging.Error("xp2p client deploy: local install failed", "err", err)
		return 1
	}
	logging.Info("xp2p client deploy: local install completed", "install_dir", installOpts.InstallDir, "config_dir", installOpts.ConfigDir)

	cancelDiagnostics := startDiagnostics(ctx, cfg.Client.DiagPort)
	if cancelDiagnostics != nil {
		defer cancelDiagnostics()
	}

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	runOpts := client.RunOptions{
		InstallDir: installOpts.InstallDir,
		ConfigDir:  installOpts.ConfigDir,
		Heartbeat: client.HeartbeatOptions{
			Enabled:      true,
			Interval:     2 * time.Second,
			Timeout:      2 * time.Second,
			Port:         cfg.Server.Port,
			SocksAddress: cfg.Client.SocksAddress,
		},
	}
	runErrCh := make(chan error, 1)
	logging.Info("xp2p client deploy: starting local client run", "install_dir", runOpts.InstallDir, "config_dir", runOpts.ConfigDir)
	go func() {
		runErrCh <- clientRunFunc(runCtx, runOpts)
	}()

	socksAddr := strings.TrimSpace(cfg.Client.SocksAddress)
	if socksAddr != "" {
		logging.Info("xp2p client deploy: waiting for local SOCKS proxy", "socks_proxy", socksAddr)
		if err := waitForSocksProxy(runCtx, socksAddr, socksReadyTimeout); err != nil {
			completionState = "FAIL socks"
			logging.Error("xp2p client deploy: socks proxy not ready", "err", err)
			if stopErr := stopLocalClient(runCancel, runErrCh); stopErr != nil {
				logging.Warn("xp2p client deploy: local client stop failed", "err", stopErr)
			}
			return 1
		}
		logging.Info("xp2p client deploy: client run active", "install_dir", runOpts.InstallDir, "config_dir", runOpts.ConfigDir)

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
			if stopErr := stopLocalClient(runCancel, runErrCh); stopErr != nil {
				logging.Warn("xp2p client deploy: local client stop failed", "err", stopErr)
			}
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
		if err := ping.Run(ctx, markerTarget, pingOpts); err != nil {
			completionState = "FAIL ping"
			logging.Error("xp2p client deploy: ping failed", "err", err)
			if stopErr := stopLocalClient(runCancel, runErrCh); stopErr != nil {
				logging.Warn("xp2p client deploy: local client stop failed", "err", stopErr)
			}
			return 1
		}
		logging.Info("xp2p client deploy: ping ok")
	} else {
		logging.Warn("xp2p client deploy: socks proxy address missing; skipping ping")
	}

	if err := waitForHeartbeat(runCtx, filepath.Join(runOpts.InstallDir, layout.ClientHeartbeatStateFileName), 10*time.Second); err != nil {
		completionState = "FAIL heartbeat"
		logging.Error("xp2p client deploy: heartbeat missing", "err", err)
		if stopErr := stopLocalClient(runCancel, runErrCh); stopErr != nil {
			logging.Warn("xp2p client deploy: local client stop failed", "err", stopErr)
		}
		return 1
	}

	if err := stopLocalClient(runCancel, runErrCh); err != nil {
		completionState = "FAIL client-stop"
		logging.Error("xp2p client deploy: client run exited", "err", err)
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
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == "install-dir" {
			installDirSet = true
		}
	})

	return deployOptions{
		manifest: manifestOptions{
			remoteHost:     host,
			installDir:     strings.TrimSpace(*installDir),
			installDirSet:  installDirSet,
			trojanPort:     serverPortValue,
			trojanUser:     strings.TrimSpace(userValue),
			trojanPassword: strings.TrimSpace(passwordValue),
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
	return client.InstallOptions{
		InstallDir:    cfg.Client.InstallDir,
		ConfigDir:     cfg.Client.ConfigDir,
		ServerAddress: link.ServerAddress,
		ServerPort:    link.ServerPort,
		User:          link.User,
		Password:      link.Password,
		ServerName:    link.ServerName,
		AllowInsecure: link.AllowInsecure,
		Force:         true,
		TunEnabled:    cfg.Client.TunEnabled,
		TunEnabledSet: true,
		TunName:       cfg.Client.TunName,
		TunMTU:        cfg.Client.TunMTU,
	}
}

func waitForSocksProxy(ctx context.Context, addr string, timeout time.Duration) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return fmt.Errorf("socks proxy address is empty")
	}

	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		conn, err := net.DialTimeout("tcp", addr, socksProbeInterval)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("socks proxy %s not ready: %w", addr, lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(socksProbeInterval):
		}
	}
}

func stopLocalClient(cancel context.CancelFunc, runErrCh <-chan error) error {
	if cancel != nil {
		cancel()
	}
	select {
	case err := <-runErrCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for local client to stop")
	}
}

func waitForHeartbeat(ctx context.Context, statePath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		state, err := heartbeat.Load(statePath)
		if err == nil && len(state.Entries) > 0 {
			return nil
		}
		if err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("heartbeat state: %w", lastErr)
	}
	return fmt.Errorf("heartbeat state %s not found", statePath)
}
