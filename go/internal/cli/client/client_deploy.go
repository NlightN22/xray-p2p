package clientcmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
)

const (
	deployLinkTTL      = 10 * time.Minute
	socksReadyTimeout  = 30 * time.Second
	socksProbeInterval = 500 * time.Millisecond
)

type manifestOptions struct {
	remoteHost     string
	installDir     string
	trojanPort     string
	trojanUser     string
	trojanPassword string
}

type runtimeOptions struct {
	remoteHost string
	deployPort string
	serverHost string
	encLink    deploylink.EncryptedLink
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
	linkURL, err := buildDeployLink(&opts)
	if err != nil {
		logging.Error("xp2p client deploy: build link failed", "err", err)
		return 2
	}
	logging.Info("xp2p client deploy: link generated", "link", linkURL)
	logging.Info("xp2p client deploy: waiting for server...", "remote_host", opts.runtime.remoteHost, "deploy_port", opts.runtime.deployPort)

	// Retry handshake until server is ready or timeout elapses.
	var (
		res          deployResult
		handshakeErr error
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
		res, handshakeErr = performDeployHandshake(ctx, opts)
		if handshakeErr == nil {
			break
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

	if res.ExitCode != 0 {
		logging.Error("xp2p client deploy: server install failed", "exit_code", res.ExitCode)
		return 1
	}
	if strings.TrimSpace(res.Link) == "" {
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
		logging.Error("xp2p client deploy: local install failed", "err", err)
		return 1
	}
	logging.Info("xp2p client deploy: local install completed", "install_dir", installOpts.InstallDir, "config_dir", installOpts.ConfigDir)

	cancelDiagnostics := startDiagnostics(ctx, cfg.Server.Port)
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
			logging.Error("xp2p client deploy: socks proxy not ready", "err", err)
			abortLocalClient(runCancel, runErrCh)
			return 1
		}

		targetHost := strings.TrimSpace(tl.ServerAddress)
		if targetHost == "" {
			targetHost = strings.TrimSpace(opts.runtime.serverHost)
		}
		if targetHost == "" {
			targetHost = strings.TrimSpace(opts.runtime.remoteHost)
		}

		logging.Info("xp2p client deploy: verifying connectivity via SOCKS ping", "target", targetHost)
		pingOpts := ping.Options{
			Count:      1,
			Timeout:    3 * time.Second,
			Proto:      "tcp",
			SocksProxy: socksAddr,
		}
		if err := ping.Run(ctx, targetHost, pingOpts); err != nil {
			logging.Error("xp2p client deploy: ping failed", "err", err)
			abortLocalClient(runCancel, runErrCh)
			return 1
		}
		logging.Info("xp2p client deploy: ping ok")
	} else {
		logging.Warn("xp2p client deploy: socks proxy address missing; skipping ping")
	}

	logging.Info("xp2p client deploy: client run active (press Ctrl+C to stop)")
	if err := <-runErrCh; err != nil && !errors.Is(err, context.Canceled) {
		logging.Error("xp2p client deploy: client run exited", "err", err)
		return 1
	}
	return 0
}

func parseDeployFlags(cfg config.Config, args []string) (deployOptions, error) {
	fs := flag.NewFlagSet("xp2p client deploy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	remoteHost := fs.String("remote-host", "", "deploy host name or address")
	deployPort := fs.String("deploy-port", "62025", "deploy port (default 62025)")
	trojanUser := fs.String("user", "", "Trojan user identifier (email)")
	trojanPassword := fs.String("password", "", "Trojan user password (auto-generated when omitted)")
	trojanPort := fs.String("trojan-port", "", "Trojan service port")

	if err := fs.Parse(args); err != nil {
		return deployOptions{}, err
	}
	if fs.NArg() > 0 {
		return deployOptions{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	host := strings.TrimSpace(*remoteHost)
	if host == "" || strings.HasPrefix(host, "-") {
		return deployOptions{}, fmt.Errorf("--remote-host is required")
	}
	if err := netutil.ValidateHost(host); err != nil {
		return deployOptions{}, fmt.Errorf("--remote-host: %v", err)
	}

	serverHostValue := firstNonEmpty(cfg.Server.Host, host)
	serverPortValue := normalizeServerPort(cfg, *trojanPort)

	userValue := strings.TrimSpace(firstNonEmpty(*trojanUser, cfg.Client.User))
	// optional: user/password can be empty; server may generate

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

	return deployOptions{
		manifest: manifestOptions{
			remoteHost:     host,
			installDir:     strings.TrimSpace(cfg.Server.InstallDir),
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
	manifest := spec.Manifest{
		Host:           strings.TrimSpace(opts.runtime.serverHost),
		Version:        2,
		InstallDir:     strings.TrimSpace(opts.manifest.installDir),
		TrojanPort:     strings.TrimSpace(opts.manifest.trojanPort),
		TrojanUser:     strings.TrimSpace(opts.manifest.trojanUser),
		TrojanPassword: strings.TrimSpace(opts.manifest.trojanPassword),
	}
	linkURL, enc, err := deploylink.Build(opts.runtime.remoteHost, opts.runtime.deployPort, manifest, deployLinkTTL)
	if err != nil {
		return "", err
	}
	opts.runtime.encLink = enc
	return linkURL, nil
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
			if lastErr != nil {
				return fmt.Errorf("socks proxy %s not ready: %w", addr, lastErr)
			}
			return fmt.Errorf("socks proxy %s not ready", addr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(socksProbeInterval):
		}
	}
}

func abortLocalClient(cancel context.CancelFunc, runErrCh <-chan error) {
	cancel()
	select {
	case err := <-runErrCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			logging.Warn("xp2p client deploy: local client run exited", "err", err)
		}
	case <-time.After(5 * time.Second):
		logging.Warn("xp2p client deploy: timed out waiting for local client to stop")
	}
}
