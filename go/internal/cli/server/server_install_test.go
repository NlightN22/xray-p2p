package servercmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerInstall(t *testing.T) {
	tests := []struct {
		name       string
		cfg        config.Config
		args       []string
		prepare    func(*testing.T) []string
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
			args: []string{
				"--path", `D:\xp2p`,
				"--config-dir", "cfg-custom",
				"--port", "65000",
				"--cert", `C:\certs\server.pem`,
				"--key", `C:\certs\server.key`,
				"--host", "custom.example.test",
				"--force",
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
			args:     []string{"--host", "bad host"},
			wantCode: 1,
			wantCall: false,
		},
		{
			name: "invalid host in manifest",
			cfg:  serverCfg(`C:\xp2p`, "config-server", ""),
			prepare: func(t *testing.T) []string {
				t.Helper()
				dir := t.TempDir()
				path := filepath.Join(dir, "deployment.json")
				manifest := spec.Manifest{
					RemoteHost:  "bad host",
					XP2PVersion: "9.9.9",
					GeneratedAt: time.Date(2025, 11, 4, 7, 47, 42, 0, time.UTC),
				}
				file, err := os.Create(path)
				if err != nil {
					t.Fatalf("create manifest: %v", err)
				}
				if err := spec.Write(file, manifest); err != nil {
					t.Fatalf("write manifest: %v", err)
				}
				if err := file.Close(); err != nil {
					t.Fatalf("close manifest: %v", err)
				}
				return []string{"--deploy-file", path}
			},
			wantCode: 1,
			wantCall: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			args := tt.args
			if tt.prepare != nil {
				args = append(args, tt.prepare(t)...)
			}
			code, calls := execInstall(tt.cfg, args, tt.host, tt.hostErr, tt.installErr)
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

func execInstall(cfg config.Config, args []string, host string, hostErr, installErr error) (int, []server.InstallOptions) {
	var calls []server.InstallOptions
	restoreInstall := stubServerInstall(func(ctx context.Context, opts server.InstallOptions) error {
		calls = append(calls, opts)
		return installErr
	})
	defer restoreInstall()
	defer stubDetectPublicHost(host, hostErr)()
	code := runServerInstall(context.Background(), cfg, args)
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
		code := runServerInstall(context.Background(), cfg, nil)
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

func TestRunServerInstallUsesManifestCredential(t *testing.T) {
	cfg := serverCfg(`C:\xp2p`, "config-server", "")
	restoreInstall := stubServerInstall(func(context.Context, server.InstallOptions) error { return nil })
	defer restoreInstall()

	var added []server.AddUserOptions
	restoreAdd := stubServerUserAdd(func(ctx context.Context, opts server.AddUserOptions) error {
		added = append(added, opts)
		return nil
	})
	defer restoreAdd()

	restoreLink := stubServerUserLink(func(ctx context.Context, opts server.UserLinkOptions) (server.UserLink, error) {
		return server.UserLink{
			UserID:   opts.UserID,
			Password: "manifest-secret",
			Link:     "trojan://manifest-link",
		}, nil
	})
	defer restoreLink()

	dir := t.TempDir()
	path := filepath.Join(dir, "deployment.json")
	manifest := spec.Manifest{
		RemoteHost:     "deploy.example",
		XP2PVersion:    "0.1.1",
		GeneratedAt:    time.Date(2025, 11, 4, 7, 47, 42, 0, time.UTC),
		TrojanUser:     "client@example",
		TrojanPassword: "manifest-secret",
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create manifest: %v", err)
	}
	if err := spec.Write(file, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close manifest: %v", err)
	}

	output := captureStdout(t, func() {
		code := runServerInstall(context.Background(), cfg, []string{"--deploy-file", path})
		if code != 0 {
			t.Fatalf("exit code: got %d want 0", code)
		}
	})

	if len(added) != 1 {
		t.Fatalf("trojan user add calls: got %d want 1", len(added))
	}
	if added[0].UserID != "client@example" {
		t.Fatalf("user id mismatch: got %q want %q", added[0].UserID, "client@example")
	}
	if added[0].Password != "manifest-secret" {
		t.Fatalf("password mismatch: got %q want %q", added[0].Password, "manifest-secret")
	}
	if !strings.Contains(output, "Deploy manifest trojan credential") {
		t.Fatalf("output missing manifest credential banner: %q", output)
	}
	if !strings.Contains(output, "client@example") || !strings.Contains(output, "manifest-secret") {
		t.Fatalf("output missing manifest credential details: %q", output)
	}
	if strings.Contains(output, "Generated trojan credential") {
		t.Fatalf("unexpected default credential generation: %q", output)
	}
}

func TestRunServerInstallGeneratesCredentialWhenManifestHasNoAuth(t *testing.T) {
	cfg := serverCfg(`C:\xp2p`, "config-server", "")
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

	dir := t.TempDir()
	path := filepath.Join(dir, "deployment.json")
	manifest := spec.Manifest{
		RemoteHost:  "deploy.example",
		XP2PVersion: "0.1.1",
		GeneratedAt: time.Date(2025, 11, 4, 7, 47, 42, 0, time.UTC),
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create manifest: %v", err)
	}
	if err := spec.Write(file, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close manifest: %v", err)
	}

	output := captureStdout(t, func() {
		code := runServerInstall(context.Background(), cfg, []string{"--deploy-file", path})
		if code != 0 {
			t.Fatalf("exit code: got %d want 0", code)
		}
	})

	if len(added) != 1 {
		t.Fatalf("trojan user add calls: got %d want 1", len(added))
	}
	if !strings.Contains(output, "Generated trojan credential") {
		t.Fatalf("output missing generated credential banner: %q", output)
	}
}
