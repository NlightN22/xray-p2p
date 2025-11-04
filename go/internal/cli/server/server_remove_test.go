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
		args     []string
		wantCode int
		wantOpts server.RemoveOptions
	}{
		{
			name:     "defaults",
			cfg:      serverCfg(`C:\xp2p`, "", ""),
			wantCode: 0,
			wantOpts: server.RemoveOptions{InstallDir: `C:\xp2p`},
		},
		{
			name:     "flags override",
			cfg:      serverCfg(`C:\xp2p`, "", ""),
			args:     []string{"--path", `D:\xp2p`, "--keep-files", "--ignore-missing"},
			wantCode: 0,
			wantOpts: server.RemoveOptions{InstallDir: `D:\xp2p`, KeepFiles: true, IgnoreMissing: true},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			code, opts := execRemove(tt.cfg, tt.args)
			if code != tt.wantCode {
				t.Fatalf("exit code: got %d want %d", code, tt.wantCode)
			}
			requireEqual(t, opts, tt.wantOpts, "remove options")
		})
	}
}

func execRemove(cfg config.Config, args []string) (int, server.RemoveOptions) {
	var captured server.RemoveOptions
	restoreInstall := stubServerInstall(nil)
	defer restoreInstall()
	restoreRemove := stubServerRemove(func(ctx context.Context, opts server.RemoveOptions) error {
		captured = opts
		return nil
	})
	defer restoreRemove()
	code := runServerRemove(context.Background(), cfg, args)
	return code, captured
}
