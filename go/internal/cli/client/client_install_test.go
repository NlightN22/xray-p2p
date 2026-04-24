package clientcmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestRunClientInstall(t *testing.T) {
	defaultCfg := config.Config{
		Client: config.ClientConfig{
			InstallDir:    `C:\xp2p-client`,
			ConfigDir:     "config-client",
			ServerAddress: "default",
			ServerPort:    "8443",
			User:          "default@example.com",
			Password:      "default-password",
			ServerName:    "default.name",
			AllowInsecure: false,
			TunEnabled:    true,
			TunName:       "xp2pc",
			TunMTU:        1500,
			TunAddr:       "198.18.0.1/30",
			TunMode:       "split",
		},
	}

	tests := []struct {
		name       string
		cfg        config.Config
		args       []string
		installErr error
		wantCode   int
		wantCalled bool
		check      func(*testing.T, client.InstallOptions)
	}{
		{
			name: "cli overrides",
			cfg:  defaultCfg,
			args: []string{
				"--path", `D:\xp2p-client`,
				"--config-dir", "cfg-client",
				"--host", "example.org",
				"--port", "9443",
				"--user", "user@example.com",
				"--password", "secret",
				"--sni", "custom.name",
				"--allow-insecure",
				"--force",
			},
			wantCode:   0,
			wantCalled: true,
			check: func(t *testing.T, opts client.InstallOptions) {
				requireEqual(t, opts, client.InstallOptions{
					InstallDir:            `D:\xp2p-client`,
					ConfigDir:             "cfg-client",
					ServerAddress:         "example.org",
					ServerPort:            "9443",
					User:                  "user@example.com",
					Password:              "secret",
					ServerName:            "custom.name",
					AllowInsecure:         true,
					PinnedPeerCertSHA256:  "",
					VerifyPeerCertByName:  "",
					AllowInsecureOverride: true,
					Force:                 true,
					TunEnabled:            true,
					TunEnabledSet:         true,
					TunName:               "xp2pc",
					TunMTU:                1500,
					TunAddr:               "198.18.0.1/30",
					TunMode:               "split",
				}, "install options")
			},
		},
		{
			name: "mode proxy disables tun",
			cfg:  defaultCfg,
			args: []string{
				"--host", "example.org",
				"--user", "user@example.com",
				"--password", "secret",
				"--mode", "proxy",
			},
			wantCode:   0,
			wantCalled: true,
			check: func(t *testing.T, opts client.InstallOptions) {
				if opts.TunEnabled {
					t.Fatalf("expected tun to be disabled in proxy mode")
				}
				if !opts.TunEnabledSet {
					t.Fatalf("expected tun enabled to be explicitly set")
				}
			},
		},
		{
			name: "mode tun full sets tun-mode",
			cfg:  defaultCfg,
			args: []string{
				"--host", "example.org",
				"--user", "user@example.com",
				"--password", "secret",
				"--mode", "tun:full",
			},
			wantCode:   0,
			wantCalled: true,
			check: func(t *testing.T, opts client.InstallOptions) {
				if !opts.TunEnabled {
					t.Fatalf("expected tun to be enabled")
				}
				if opts.TunMode != "full" || !opts.TunModeSet {
					t.Fatalf("expected tun mode full (set=%v), got %q", opts.TunModeSet, opts.TunMode)
				}
			},
		},
		{
			name: "proxy mode rejects tun-mode flag",
			cfg:  defaultCfg,
			args: []string{
				"--host", "example.org",
				"--user", "user@example.com",
				"--password", "secret",
				"--mode", "proxy",
				"--tun-mode", "full",
			},
			wantCode:   2,
			wantCalled: false,
		},
		{
			name: "sni does not enable allow insecure",
			cfg:  defaultCfg,
			args: []string{
				"--host", "example.org",
				"--user", "user@example.com",
				"--password", "secret",
				"--sni", "custom.name",
			},
			wantCode:   0,
			wantCalled: true,
			check: func(t *testing.T, opts client.InstallOptions) {
				if opts.AllowInsecure {
					t.Fatalf("expected allow insecure to remain false when only sni is provided")
				}
				if opts.AllowInsecureOverride {
					t.Fatalf("expected allow insecure override to remain false when only sni is provided")
				}
			},
		},
		{
			name:       "install error surfaces",
			cfg:        defaultCfg,
			args:       []string{"--host", "host", "--user", "user@example.com", "--password", "secret"},
			installErr: errors.New("install failure"),
			wantCode:   1,
			wantCalled: true,
		},
		{
			name: "install from link",
			cfg:  defaultCfg,
			args: []string{
				"--link", "trojan://secret@links.example.test:58443?alpn=h2,http/1.1&pinnedPeerCertSha256=deadbeef&security=tls&sni=links.example.test&verifyPeerCertByName=links.example.test#alpha@example.com",
			},
			wantCode:   0,
			wantCalled: true,
			check: func(t *testing.T, opts client.InstallOptions) {
				if opts.ServerAddress != "links.example.test" {
					t.Fatalf("unexpected server address: %s", opts.ServerAddress)
				}
				if opts.ServerPort != "58443" {
					t.Fatalf("unexpected server port: %s", opts.ServerPort)
				}
				if opts.User != "alpha@example.com" {
					t.Fatalf("unexpected user: %s", opts.User)
				}
				if opts.Password != "secret" {
					t.Fatalf("unexpected password: %s", opts.Password)
				}
				if opts.ServerName != "links.example.test" {
					t.Fatalf("unexpected server name: %s", opts.ServerName)
				}
				if opts.AllowInsecure {
					t.Fatalf("expected allow insecure to remain false")
				}
				if opts.PinnedPeerCertSHA256 != "deadbeef" {
					t.Fatalf("unexpected pinned hash: %s", opts.PinnedPeerCertSHA256)
				}
				if opts.VerifyPeerCertByName != "links.example.test" {
					t.Fatalf("unexpected verify peer name: %s", opts.VerifyPeerCertByName)
				}
				if len(opts.ALPN) != 2 || opts.ALPN[0] != "h2" || opts.ALPN[1] != "http/1.1" {
					t.Fatalf("unexpected alpn: %v", opts.ALPN)
				}
			},
		},
		{
			name: "link email query",
			cfg:  config.Config{},
			args: []string{
				"--link", "trojan://secret@links.example.test:58443?pinnedPeerCertSha256=deadbeef&email=alpha@example.com&security=tls&sni=links.example.test&verifyPeerCertByName=links.example.test",
			},
			wantCode:   0,
			wantCalled: true,
			check: func(t *testing.T, opts client.InstallOptions) {
				if opts.ServerAddress != "links.example.test" {
					t.Fatalf("unexpected server address: %s", opts.ServerAddress)
				}
				if opts.User != "alpha@example.com" {
					t.Fatalf("unexpected user: %s", opts.User)
				}
			},
		},
		{
			name: "link user decoding",
			cfg:  config.Config{},
			args: []string{
				"--link", "trojan://secret@links.example.test:58443#alpha%40example.com",
			},
			wantCode:   0,
			wantCalled: true,
			check: func(t *testing.T, opts client.InstallOptions) {
				if opts.User != "alpha@example.com" {
					t.Fatalf("unexpected user: %s", opts.User)
				}
			},
		},
		{
			name: "requires user without link",
			cfg: config.Config{
				Client: config.ClientConfig{
					InstallDir:    `C:\xp2p-client`,
					ConfigDir:     "config-client",
					ServerAddress: "host",
					ServerPort:    "8443",
					User:          "from-config@example.com",
					Password:      "secret",
				},
			},
			args:       []string{"--host", "example.org", "--password", "secret"},
			wantCode:   2,
			wantCalled: false,
		},
		{
			name:       "requires server address without link",
			cfg:        clientCfg(`C:\xp2p-client`, "config-client"),
			args:       []string{"--user", "alpha@example.com", "--password", "secret"},
			wantCode:   2,
			wantCalled: false,
		},
		{
			name:       "requires password without link",
			cfg:        clientCfg(`C:\xp2p-client`, "config-client"),
			args:       []string{"--host", "example.org", "--user", "alpha@example.com"},
			wantCode:   2,
			wantCalled: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			code, calls := execClientInstall(tt.cfg, tt.args, tt.installErr)
			if code != tt.wantCode {
				t.Fatalf("exit code: got %d want %d", code, tt.wantCode)
			}
			if tt.wantCalled != (len(calls) == 1) {
				t.Fatalf("install called=%v want %v", len(calls) == 1, tt.wantCalled)
			}
			if tt.wantCalled && tt.check != nil {
				tt.check(t, calls[0])
			}
		})
	}
}

func execClientInstall(cfg config.Config, args []string, installErr error) (int, []client.InstallOptions) {
	var calls []client.InstallOptions
	restore := stubClientInstall(func(ctx context.Context, opts client.InstallOptions) error {
		calls = append(calls, opts)
		return installErr
	})
	defer restore()
	defer stubTunPreflight(nil)()

	code := runClientInstall(context.Background(), cfg, args)
	return code, calls
}

func TestRunClientInstallRejectsTunModeConflictWithoutForce(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)

	configPath := config.ConfigPath(layout.ClientConfigFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("[client]\n  tun_mode = \"split\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := clientCfg(`C:\xp2p-client`, "config-client")
	args := []string{
		"--host", "example.org",
		"--user", "user@example.com",
		"--password", "secret",
		"--tun-mode", "full",
	}

	code, calls := execClientInstall(cfg, args, nil)
	if code != 1 {
		t.Fatalf("exit code: got %d want %d", code, 1)
	}
	if len(calls) != 0 {
		t.Fatalf("expected install to be skipped")
	}
}
