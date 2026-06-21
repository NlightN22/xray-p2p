package clientcmd

import (
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

func TestParseDeployFlagsPopulatesOptions(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{
			Host: "srv.example.com",
			Port: "58443",
		},
		Client: config.ClientConfig{
			User:     "default@example.com",
			Password: "default-pass",
		},
	}

	args := []string{
		"--host", "deploy.example.com",
		"--port", "62030",
		"--user", "branch@example.com",
		"--password", "secret",
		"--trojan-port", "65010",
		"--tun-mode", "full",
		"--force",
	}

	opts, err := parseDeployFlags(cfg, args)
	if err != nil {
		t.Fatalf("parseDeployFlags returned error: %v", err)
	}
	if opts.runtime.remoteHost != "deploy.example.com" {
		t.Fatalf("runtime remote host = %s", opts.runtime.remoteHost)
	}
	if opts.runtime.deployPort != "62030" {
		t.Fatalf("runtime deploy port = %s", opts.runtime.deployPort)
	}
	if opts.runtime.serverHost != "srv.example.com" {
		t.Fatalf("runtime server host = %s", opts.runtime.serverHost)
	}
	if opts.manifest.trojanPort != "65010" {
		t.Fatalf("manifest trojan port = %s", opts.manifest.trojanPort)
	}
	if opts.manifest.trojanUser != "branch@example.com" {
		t.Fatalf("manifest user = %s", opts.manifest.trojanUser)
	}
	if opts.manifest.trojanPassword != "secret" {
		t.Fatalf("manifest password = %s", opts.manifest.trojanPassword)
	}
	if !opts.manifest.tunModeSet || opts.manifest.tunMode != "full" {
		t.Fatalf("expected tun mode full, got %q (set=%v)", opts.manifest.tunMode, opts.manifest.tunModeSet)
	}
	if !opts.manifest.force {
		t.Fatalf("expected force to be set")
	}
}

func TestParseDeployFlagsModeProxy(t *testing.T) {
	opts, err := parseDeployFlags(config.Config{}, []string{
		"--host", "deploy.example.com",
		"--mode", "proxy",
	})
	if err != nil {
		t.Fatalf("parseDeployFlags returned error: %v", err)
	}
	if !opts.manifest.mode.set || opts.manifest.mode.tunEnabled {
		t.Fatalf("expected proxy mode to be set, got set=%v tun=%v", opts.manifest.mode.set, opts.manifest.mode.tunEnabled)
	}
}

func TestParseDeployFlagsRejectsTunModeWithProxyMode(t *testing.T) {
	_, err := parseDeployFlags(config.Config{}, []string{
		"--host", "deploy.example.com",
		"--mode", "proxy",
		"--tun-mode", "full",
	})
	if err == nil {
		t.Fatalf("expected error for tun-mode with proxy mode")
	}
}

func TestParseDeployFlagsModeTunFullShorthand(t *testing.T) {
	opts, err := parseDeployFlags(config.Config{}, []string{
		"--host", "deploy.example.com",
		"--mode", "tun:full",
	})
	if err != nil {
		t.Fatalf("parseDeployFlags returned error: %v", err)
	}
	if !opts.manifest.mode.set || !opts.manifest.mode.tunEnabled {
		t.Fatalf("expected tun mode to be set, got set=%v tun=%v", opts.manifest.mode.set, opts.manifest.mode.tunEnabled)
	}
	if !opts.manifest.tunModeSet || opts.manifest.tunMode != "full" {
		t.Fatalf("expected tun mode full, got %q (set=%v)", opts.manifest.tunMode, opts.manifest.tunModeSet)
	}
}

func TestParseDeployFlagsRequiresRemoteHost(t *testing.T) {
	_, err := parseDeployFlags(config.Config{}, []string{"--user", "demo", "--password", "secret"})
	if err == nil {
		t.Fatalf("expected error for missing remote host")
	}
}

func TestParseDeployFlagsInstallDirFlag(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: `C:\xp2p-default`,
		},
	}

	opts, err := parseDeployFlags(cfg, []string{"--host", "deploy.example.com"})
	if err != nil {
		t.Fatalf("parseDeployFlags returned error: %v", err)
	}
	if opts.manifest.installDir != "" || opts.manifest.installDirSet {
		t.Fatalf("expected install dir to be unset, got %q (set=%v)", opts.manifest.installDir, opts.manifest.installDirSet)
	}

	opts, err = parseDeployFlags(cfg, []string{"--host", "deploy.example.com", "--install-dir", `D:\xp2p-custom`})
	if err != nil {
		t.Fatalf("parseDeployFlags returned error: %v", err)
	}
	if !opts.manifest.installDirSet || opts.manifest.installDir != `D:\xp2p-custom` {
		t.Fatalf("expected install dir flag to be set, got %q (set=%v)", opts.manifest.installDir, opts.manifest.installDirSet)
	}
}

func TestParseDeployFlagsRejectsInvalidPassword(t *testing.T) {
	_, err := parseDeployFlags(config.Config{}, []string{
		"--host", "deploy.example.com",
		"--user", "branch@example.com",
		"--password", "bad+pass",
	})
	if err == nil {
		t.Fatalf("expected error for invalid password")
	}
}

func TestParseDeployFlagsRejectsInvalidTunMode(t *testing.T) {
	_, err := parseDeployFlags(config.Config{}, []string{
		"--host", "deploy.example.com",
		"--tun-mode", "fast",
	})
	if err == nil {
		t.Fatalf("expected error for invalid tun mode")
	}
}

func TestBuildInstallOptionsFromLinkUsesConfigDefaults(t *testing.T) {
	cfg := config.Config{
		Client: config.ClientConfig{
			InstallDir: `C:\xp2p`,
			ConfigDir:  "cfg-client",
			TunEnabled: true,
			TunName:    "xp2pc",
			TunMTU:     1500,
			TunAddr:    "198.18.0.1/30",
		},
	}

	opts := buildInstallOptionsFromLink(cfg, trojanLink{
		Endpoint: tunnel.Endpoint{
			Host:       "edge.example.com",
			Port:       58443,
			ServerName: "edge.example.com",
			TLS: tunnel.TLSMetadata{
				AllowInsecure:        true,
				PinnedPeerCertSHA256: "deadbeef",
				VerifyPeerCertByName: "edge.example.com",
			},
		},
		User: tunnel.User{UserLabel: "user@example.com", Credential: "secret"},
	})

	if opts.InstallDir != `C:\xp2p` || opts.ConfigDir != "cfg-client" {
		t.Fatalf("unexpected install paths: %+v", opts)
	}
	if opts.ServerAddress != "edge.example.com" || opts.ServerPort != "58443" {
		t.Fatalf("unexpected target: %+v", opts)
	}
	if opts.AllowInsecure {
		t.Fatalf("expected allow insecure to be disabled when pin is set")
	}
	if opts.PinnedPeerCertSHA256 != "deadbeef" {
		t.Fatalf("expected pinned sha256, got %q", opts.PinnedPeerCertSHA256)
	}
	if opts.VerifyPeerCertByName != "edge.example.com" {
		t.Fatalf("expected verify peer name, got %q", opts.VerifyPeerCertByName)
	}
	if !opts.TunEnabled || !opts.TunEnabledSet {
		t.Fatalf("expected tun enabled defaults")
	}
	if opts.TunName != "xp2pc" || opts.TunMTU != 1500 || opts.TunAddr != "198.18.0.1/30" {
		t.Fatalf("unexpected tun defaults: %+v", opts)
	}
}
