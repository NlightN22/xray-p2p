package servercmd

import (
	"context"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerRemove(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.Config
		opts     serverRemoveCommandOptions
		wantCode int
		wantOpts server.RemoveOptions
	}{
		{
			name:     "defaults",
			cfg:      serverCfg(`C:\xp2p`, server.DefaultServerConfigDir, ""),
			wantCode: 0,
			wantOpts: server.RemoveOptions{
				InstallDir: `C:\xp2p`,
				ConfigDir:  server.DefaultServerConfigDir,
			},
		},
		{
			name:     "flags override",
			cfg:      serverCfg(`C:\xp2p`, server.DefaultServerConfigDir, ""),
			opts:     serverRemoveCommandOptions{Path: `D:\xp2p`, ConfigDir: "srv-cfg", KeepFiles: true, IgnoreMissing: true},
			wantCode: 0,
			wantOpts: server.RemoveOptions{
				InstallDir:    `D:\xp2p`,
				ConfigDir:     "srv-cfg",
				KeepFiles:     true,
				IgnoreMissing: true,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			code, opts := execRemove(tt.cfg, tt.opts)
			if code != tt.wantCode {
				t.Fatalf("exit code: got %d want %d", code, tt.wantCode)
			}
			requireEqual(t, opts, tt.wantOpts, "remove options")
		})
	}
}

func TestRunServerRemovePromptDecline(t *testing.T) {
	cfg := serverCfg(`C:\xp2p`, server.DefaultServerConfigDir, "")
	var called bool
	restoreRemove := stubServerRemove(func(ctx context.Context, opts server.RemoveOptions) error {
		called = true
		return nil
	})
	defer restoreRemove()
	restorePrompt := stubPromptYesNo(false, nil)
	defer restorePrompt()

	code := runServerRemove(context.Background(), cfg, serverRemoveCommandOptions{})
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if called {
		t.Fatalf("server remove should not run when prompt is declined")
	}
}

func TestRunServerRemoveQuietSkipsPrompt(t *testing.T) {
	cfg := serverCfg(`C:\xp2p`, server.DefaultServerConfigDir, "")
	var called bool
	restoreRemove := stubServerRemove(func(ctx context.Context, opts server.RemoveOptions) error {
		called = true
		return nil
	})
	defer restoreRemove()
	restorePrompt := stubPromptYesNo(false, nil)
	defer restorePrompt()

	code := runServerRemove(context.Background(), cfg, serverRemoveCommandOptions{Quiet: true})
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	if !called {
		t.Fatalf("server remove should proceed in quiet mode")
	}
}

func execRemove(cfg config.Config, opts serverRemoveCommandOptions) (int, server.RemoveOptions) {
	var captured server.RemoveOptions
	restoreInstall := stubServerInstall(nil)
	defer restoreInstall()
	restoreRemove := stubServerRemove(func(ctx context.Context, opts server.RemoveOptions) error {
		captured = opts
		return nil
	})
	defer restoreRemove()
	restorePrompt := stubPromptYesNo(true, nil)
	defer restorePrompt()
	code := runServerRemove(context.Background(), cfg, opts)
	return code, captured
}
