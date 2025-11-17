package clientcmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestRunClientRun(t *testing.T) {
	tests := []struct {
		name      string
		cfg       func(*testing.T) config.Config
		args      []string
		prepared  bool
		autoPrep  bool
		wantCode  int
		wantInst  int
		wantRun   int
		onInstall func(*testing.T, config.Config, client.InstallOptions)
		onRun     func(*testing.T, config.Config, client.RunOptions)
	}{
		{
			name: "auto install when missing",
			cfg: func(t *testing.T) config.Config {
				return config.Config{
					Client: config.ClientConfig{
						InstallDir:    filepath.Join(t.TempDir(), "client"),
						ConfigDir:     client.DefaultClientConfigDir,
						ServerAddress: "example.org",
						ServerPort:    "9443",
						User:          "client@example.com",
						Password:      "secret",
						ServerName:    "sni.example.org",
						AllowInsecure: false,
					},
				}
			},
			args:     []string{"--auto-install"},
			autoPrep: true,
			wantCode: 0,
			wantInst: 1,
			wantRun:  1,
			onInstall: func(t *testing.T, cfg config.Config, opts client.InstallOptions) {
				if opts.InstallDir != cfg.Client.InstallDir {
					t.Fatalf("unexpected install dir: %s", opts.InstallDir)
				}
				if opts.ServerAddress != cfg.Client.ServerAddress {
					t.Fatalf("unexpected server address: %s", opts.ServerAddress)
				}
				if opts.User != cfg.Client.User {
					t.Fatalf("unexpected user: %s", opts.User)
				}
				if opts.Password != cfg.Client.Password {
					t.Fatalf("unexpected password: %s", opts.Password)
				}
			},
		},
		{
			name: "passes log file",
			cfg: func(t *testing.T) config.Config {
				dir := filepath.Join(t.TempDir(), "client")
				prepareClientInstallation(t, dir, client.DefaultClientConfigDir)
				return config.Config{
					Client: config.ClientConfig{
						InstallDir:    dir,
						ConfigDir:     client.DefaultClientConfigDir,
						SocksAddress:  "127.0.0.1:5555",
						ServerAddress: "edge.example.org",
					},
					Server: config.ServerConfig{
						Port: "63000",
					},
				}
			},
			args:     []string{"--xray-log-file", `logs\xray.err`},
			prepared: true,
			wantCode: 0,
			wantRun:  1,
			onRun: func(t *testing.T, _ config.Config, opts client.RunOptions) {
				if opts.ErrorLogPath != `logs\xray.err` {
					t.Fatalf("unexpected error log path: %s", opts.ErrorLogPath)
				}
				if !opts.Heartbeat.Enabled {
					t.Fatalf("heartbeat should be enabled by default")
				}
				if opts.Heartbeat.Port != "63000" {
					t.Fatalf("unexpected heartbeat port: %s", opts.Heartbeat.Port)
				}
				if opts.Heartbeat.SocksAddress != "127.0.0.1:5555" {
					t.Fatalf("unexpected heartbeat socks: %s", opts.Heartbeat.SocksAddress)
				}
			},
		},
		{
			name: "quiet missing installation",
			cfg: func(t *testing.T) config.Config {
				return config.Config{
					Client: config.ClientConfig{
						InstallDir: filepath.Join(t.TempDir(), "client"),
						ConfigDir:  client.DefaultClientConfigDir,
					},
				}
			},
			args:     []string{"--quiet"},
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg(t)
			if tt.prepared {
				prepareClientInstallation(t, cfg.Client.InstallDir, cfg.Client.ConfigDir)
			}
			code, installs, runs := execClientRun(t, cfg, tt.args, tt.autoPrep, tt.onInstall, tt.onRun)
			if code != tt.wantCode {
				t.Fatalf("exit code: got %d want %d", code, tt.wantCode)
			}
			if len(installs) != tt.wantInst {
				t.Fatalf("install call count: got %d want %d", len(installs), tt.wantInst)
			}
			if len(runs) != tt.wantRun {
				t.Fatalf("run call count: got %d want %d", len(runs), tt.wantRun)
			}
		})
	}
}

func execClientRun(t *testing.T, cfg config.Config, args []string, autoPrep bool, onInstall func(*testing.T, config.Config, client.InstallOptions), onRun func(*testing.T, config.Config, client.RunOptions)) (int, []client.InstallOptions, []client.RunOptions) {
	var installs []client.InstallOptions
	restoreInstall := stubClientInstall(func(ctx context.Context, opts client.InstallOptions) error {
		installs = append(installs, opts)
		if onInstall != nil {
			onInstall(t, cfg, opts)
		}
		if autoPrep {
			prepareClientInstallation(t, opts.InstallDir, opts.ConfigDir)
		}
		return nil
	})
	defer restoreInstall()

	var runs []client.RunOptions
	restoreRun := stubClientRun(func(ctx context.Context, opts client.RunOptions) error {
		runs = append(runs, opts)
		if onRun != nil {
			onRun(t, cfg, opts)
		}
		return nil
	})
	defer restoreRun()

	code := runClientRun(context.Background(), cfg, args)
	return code, installs, runs
}
