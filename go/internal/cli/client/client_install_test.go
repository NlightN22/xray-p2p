package clientcmd

import (
	"context"
	"errors"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
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
					InstallDir:    `D:\xp2p-client`,
					ConfigDir:     "cfg-client",
					ServerAddress: "example.org",
					ServerPort:    "9443",
					User:          "user@example.com",
					Password:      "secret",
					ServerName:    "custom.name",
					AllowInsecure: true,
					Force:         true,
				}, "install options")
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
				"--link", "trojan://secret@links.example.test:62022?allowInsecure=1&security=tls&sni=links.example.test#alpha@example.com",
			},
			wantCode:   0,
			wantCalled: true,
			check: func(t *testing.T, opts client.InstallOptions) {
				if opts.ServerAddress != "links.example.test" {
					t.Fatalf("unexpected server address: %s", opts.ServerAddress)
				}
				if opts.ServerPort != "62022" {
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
				if !opts.AllowInsecure {
					t.Fatalf("expected allow insecure from link")
				}
			},
		},
		{
			name: "link email query",
			cfg:  config.Config{},
			args: []string{
				"--link", "trojan://secret@links.example.test:62022?allowInsecure=1&email=alpha@example.com",
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
				"--link", "trojan://secret@links.example.test:62022#alpha%40example.com",
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

	code := runClientInstall(context.Background(), cfg, args)
	return code, calls
}
