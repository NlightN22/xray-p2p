package servercmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
			code, calls := execInstall(tt.cfg, tt.args, tt.host, tt.hostErr, tt.installErr)
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
