package servercmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerInstall(t *testing.T) {
	tests := []struct {
		name       string
		cfg        config.Config
		opts       serverInstallCommandOptions
		host       string
		hostErr    error
		installErr error
		wantCode   int
		wantCall   bool
		check      func(*testing.T, server.InstallOptions)
	}{
		{
			name: "cli overrides",
			cfg:  serverCfg(`C:\programdata\xp2p`, "config-server", ""),
			opts: serverInstallCommandOptions{
				Path:      `D:\xp2p`,
				ConfigDir: "cfg-custom",
				Port:      "65000",
				Cert:      `C:\certs\server.pem`,
				Key:       `C:\certs\server.key`,
				Host:      "custom.example.test",
				Force:     true,
			},
			wantCode: 0,
			wantCall: true,
			check: func(t *testing.T, opts server.InstallOptions) {
				requireEqual(t, opts, server.InstallOptions{
					InstallDir:      `D:\xp2p`,
					ConfigDir:       "cfg-custom",
					Port:            "65000",
					CertificateFile: `C:\certs\server.pem`,
					KeyFile:         `C:\certs\server.key`,
					Host:            "custom.example.test",
					Force:           true,
				}, "install options")
			},
		},
		{
			name:       "install error surfaces",
			cfg:        serverCfg(`C:\xp2p`, "config-server", ""),
			host:       "198.51.100.10",
			installErr: errors.New("boom"),
			wantCode:   1,
			wantCall:   true,
		},
		{
			name:     "host detection failure aborts",
			cfg:      serverCfg("", "", ""),
			hostErr:  errors.New("no host"),
			wantCode: 1,
			wantCall: false,
		},
		{
			name:     "invalid host flag",
			cfg:      serverCfg(`C:\xp2p`, "config-server", ""),
			opts:     serverInstallCommandOptions{Host: "bad host"},
			wantCode: 1,
			wantCall: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			code, calls := execInstall(tt.cfg, tt.opts, tt.host, tt.hostErr, tt.installErr)
			if code != tt.wantCode {
				t.Fatalf("exit code: got %d want %d", code, tt.wantCode)
			}
			if tt.wantCall != (len(calls) == 1) {
				t.Fatalf("install called=%v want %v", len(calls) == 1, tt.wantCall)
			}
			if tt.wantCall && tt.check != nil {
				tt.check(t, calls[0])
			}
		})
	}
}

func execInstall(cfg config.Config, opts serverInstallCommandOptions, host string, hostErr, installErr error) (int, []server.InstallOptions) {
	var calls []server.InstallOptions
	restoreInstall := stubServerInstall(func(ctx context.Context, opts server.InstallOptions) error {
		calls = append(calls, opts)
		return installErr
	})
	defer restoreInstall()
	defer stubDetectPublicHost(host, hostErr)()
	code := runServerInstall(context.Background(), cfg, opts)
	return code, calls
}

func TestRunServerInstallGeneratesCredentialWhenMissing(t *testing.T) {
	cfg := serverCfg(`C:\xp2p`, "config-server", "198.51.100.10")
	restoreInstall := stubServerInstall(func(context.Context, server.InstallOptions) error { return nil })
	defer restoreInstall()

	var added []server.AddUserOptions
	restoreAdd := stubServerUserAdd(func(ctx context.Context, opts server.AddUserOptions) error {
		added = append(added, opts)
		return nil
	})
	defer restoreAdd()

	restoreLink := stubServerUserLink(func(ctx context.Context, opts server.UserLinkOptions) (server.UserLink, error) {
		password := ""
		for i := len(added) - 1; i >= 0; i-- {
			if strings.EqualFold(added[i].UserID, opts.UserID) {
				password = added[i].Password
				break
			}
		}
		return server.UserLink{
			UserID:   opts.UserID,
			Password: password,
			Link:     "trojan://generated-link",
		}, nil
	})
	defer restoreLink()

	output := captureStdout(t, func() {
		code := runServerInstall(context.Background(), cfg, serverInstallCommandOptions{})
		if code != 0 {
			t.Fatalf("exit code: got %d want 0", code)
		}
	})

	if len(added) != 1 {
		t.Fatalf("trojan user add calls: got %d want 1", len(added))
	}
	if strings.TrimSpace(added[0].UserID) == "" {
		t.Fatalf("generated user id is empty")
	}
	if strings.TrimSpace(added[0].Password) == "" {
		t.Fatalf("generated password is empty")
	}
	if !strings.Contains(output, "Generated trojan credential") {
		t.Fatalf("output missing generated credential banner: %q", output)
	}
	if !strings.Contains(output, added[0].UserID) {
		t.Fatalf("output missing generated user id: %q", output)
	}
	if !strings.Contains(output, added[0].Password) {
		t.Fatalf("output missing generated password: %q", output)
	}
}
