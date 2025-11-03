package cli

import (
	"context"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestRunClientRemove(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.Config
		args     []string
		wantCode int
		wantOpts client.RemoveOptions
	}{
		{
			name:     "defaults",
			cfg:      clientCfg(`C:\xp2p-client`, ""),
			wantCode: 0,
			wantOpts: client.RemoveOptions{InstallDir: `C:\xp2p-client`},
		},
		{
			name:     "flags override",
			cfg:      clientCfg(`C:\xp2p-client`, ""),
			args:     []string{"--path", `D:\xp2p-client`, "--keep-files", "--ignore-missing"},
			wantCode: 0,
			wantOpts: client.RemoveOptions{
				InstallDir:    `D:\xp2p-client`,
				KeepFiles:     true,
				IgnoreMissing: true,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			code, opts := execClientRemove(t, tt.cfg, tt.args)
			if code != tt.wantCode {
				t.Fatalf("exit code: got %d want %d", code, tt.wantCode)
			}
			requireEqual(t, opts, tt.wantOpts, "remove options")
		})
	}
}

func execClientRemove(t *testing.T, cfg config.Config, args []string) (int, client.RemoveOptions) {
	var captured client.RemoveOptions
	restoreInstall := stubClientInstall(nil)
	defer restoreInstall()
	restoreRemove := stubClientRemove(func(ctx context.Context, opts client.RemoveOptions) error {
		captured = opts
		return nil
	})
	defer restoreRemove()

	code := runClientRemove(context.Background(), cfg, args)
	return code, captured
}
