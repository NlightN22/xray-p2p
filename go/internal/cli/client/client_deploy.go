package clientcmd

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
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
	// v2 encryption payload
	encCT    []byte
	encKey   string
	encNonce string
	encExp   int64
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
	link := buildDeployLink(&opts)
	logging.Info("xp2p client deploy: link generated", "link", link)
	logging.Info("xp2p client deploy: waiting for server…", "remote_host", opts.runtime.remoteHost, "deploy_port", opts.runtime.deployPort)

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

	// Verify via SOCKS ping
	logging.Info("xp2p client deploy: verifying connectivity via SOCKS ping")
	pingOpts := ping.Options{
		Count:      1,
		Timeout:    3 * time.Second,
		Proto:      "tcp",
		SocksProxy: cfg.Client.SocksAddress,
	}
	if err := ping.Run(ctx, "127.0.0.1", pingOpts); err != nil {
		logging.Error("xp2p client deploy: ping failed", "err", err)
		return 1
	}
	logging.Info("xp2p client deploy: ping ok")
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

// buildDeployLink composes a basic xp2p+deploy link.
func buildDeployLink(opts *deployOptions) string {
	host := strings.TrimSpace(opts.runtime.remoteHost)
	port := strings.TrimSpace(opts.runtime.deployPort)
	if port == "" {
		port = "62025"
	}
	// v2 encrypted manifest
	// Prepare manifest JSON
	manifest := fmt.Sprintf(`{"host":"%s","version":2,"trojan_port":"%s","install_dir":"%s","user":"%s","password":"%s","exp":%d}`,
		strings.TrimSpace(opts.runtime.serverHost),
		strings.TrimSpace(opts.manifest.trojanPort),
		strings.TrimSpace(opts.manifest.installDir),
		strings.TrimSpace(opts.manifest.trojanUser),
		strings.TrimSpace(opts.manifest.trojanPassword),
		nowPlusMinutes(10),
	)
	keyB64, keyRaw, _ := generateAESKey()
	nonceB64, nonceRaw, _ := generateNonce()
	ct, _ := encryptManifestAESGCM(keyRaw, nonceRaw, []byte(manifest))
	ctB64 := base64.RawURLEncoding.EncodeToString(ct)

	opts.runtime.encCT = ct
	opts.runtime.encKey = keyB64
	opts.runtime.encNonce = nonceB64
	opts.runtime.encExp = nowPlusMinutes(10)

	// Build v2 link: include key only in link, not sent over network
	return fmt.Sprintf("xp2p+deploy://%s:%s?v=2&k=%s&ct=%s&n=%s&exp=%d",
		host, port, keyB64, ctB64, nonceB64, opts.runtime.encExp,
	)
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
