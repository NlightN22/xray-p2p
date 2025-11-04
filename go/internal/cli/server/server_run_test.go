package servercmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerRun(t *testing.T) {
	tests := []struct {
		name      string
		cfg       func(*testing.T) config.Config
		args      []string
		prepared  bool
		autoPrep  bool
		host      string
		wantCode  int
		wantInst  int
		wantRun   int
		onInstall func(*testing.T, config.Config, server.InstallOptions)
		onRun     func(*testing.T, config.Config, server.RunOptions)
	}{
		{
			name: "uses existing installation",
			cfg: func(t *testing.T) config.Config {
				return serverCfg(filepath.Join(t.TempDir(), "srv"), server.DefaultServerConfigDir, "")
			},
			prepared: true,
			wantCode: 0,
			wantRun:  1,
			onRun: func(t *testing.T, cfg config.Config, opts server.RunOptions) {
				if opts.InstallDir != cfg.Server.InstallDir || opts.ConfigDir != cfg.Server.ConfigDir || opts.ErrorLogPath != "" {
					t.Fatalf("unexpected run options: %#v", opts)
				}
			},
		},
		{
			name: "auto install when missing",
			cfg: func(t *testing.T) config.Config {
				return serverCfg(filepath.Join(t.TempDir(), "srv"), "config-server", "")
			},
			args:     []string{"--auto-install"},
			autoPrep: true,
			host:     "198.51.100.20",
			wantCode: 0,
			wantInst: 1,
			wantRun:  1,
			onInstall: func(t *testing.T, cfg config.Config, opts server.InstallOptions) {
				if opts.InstallDir != cfg.Server.InstallDir || opts.ConfigDir != cfg.Server.ConfigDir {
					t.Fatalf("unexpected install options: %#v", opts)
				}
			},
			onRun: func(t *testing.T, _ config.Config, opts server.RunOptions) {
				if opts.ErrorLogPath != "" {
					t.Fatalf("unexpected error log path: %s", opts.ErrorLogPath)
				}
			},
		},
		{
			name: "passes log file",
			cfg: func(t *testing.T) config.Config {
				return serverCfg(filepath.Join(t.TempDir(), "srv"), server.DefaultServerConfigDir, "")
			},
			args:     []string{"--xray-log-file", `logs\xray.err`},
			prepared: true,
			wantCode: 0,
			wantRun:  1,
			onRun: func(t *testing.T, _ config.Config, opts server.RunOptions) {
				if opts.ErrorLogPath != `logs\xray.err` {
					t.Fatalf("unexpected error log path: %s", opts.ErrorLogPath)
				}
			},
		},
		{
			name: "quiet missing installation",
			cfg: func(t *testing.T) config.Config {
				return serverCfg(filepath.Join(t.TempDir(), "srv"), server.DefaultServerConfigDir, "")
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
				prepareInstallation(t, cfg.Server.InstallDir, cfg.Server.ConfigDir)
			}
			code, installs, runs := execRun(t, cfg, tt.args, tt.host, tt.autoPrep, tt.onInstall, tt.onRun)
			if code != tt.wantCode {
				t.Fatalf("exit code: got %d want %d", code, tt.wantCode)
			}
			if len(installs) != tt.wantInst {
				t.Fatalf("install count: got %d want %d", len(installs), tt.wantInst)
			}
			if len(runs) != tt.wantRun {
				t.Fatalf("run count: got %d want %d", len(runs), tt.wantRun)
			}
		})
	}
}

func execRun(t *testing.T, cfg config.Config, args []string, host string, autoPrep bool, onInstall func(*testing.T, config.Config, server.InstallOptions), onRun func(*testing.T, config.Config, server.RunOptions)) (int, []server.InstallOptions, []server.RunOptions) {
	var installs []server.InstallOptions
	restoreInstall := stubServerInstall(func(ctx context.Context, opts server.InstallOptions) error {
		installs = append(installs, opts)
		if onInstall != nil {
			onInstall(t, cfg, opts)
		}
		if autoPrep {
			prepareInstallation(t, opts.InstallDir, opts.ConfigDir)
		}
		return nil
	})
	defer restoreInstall()

	var runs []server.RunOptions
	restoreRun := stubServerRun(func(ctx context.Context, opts server.RunOptions) error {
		runs = append(runs, opts)
		if onRun != nil {
			onRun(t, cfg, opts)
		}
		return nil
	})
	defer restoreRun()

	defer stubDetectPublicHost(host, nil)()
	code := runServerRun(context.Background(), cfg, args)
	return code, installs, runs
}
