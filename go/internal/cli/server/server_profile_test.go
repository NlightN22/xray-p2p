package servercmd

import (
	"context"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func TestServerProfileCommandUpdatesProfile(t *testing.T) {
	cfg := serverCfg(`C:\xp2p`, "config-server", "example.test")
	var captured server.SetProfileOptions
	restore := stubServerSetProfile(func(_ context.Context, opts server.SetProfileOptions) (server.SetProfileResult, error) {
		captured = opts
		return server.SetProfileResult{Profile: opts.Profile, Apply: xraylive.RuntimeApplyApplied}, nil
	})
	defer restore()

	code := runServerProfile(context.Background(), cfg, []string{"vless-tls-vision"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if captured.Profile != "vless-tls-vision" {
		t.Fatalf("profile mismatch: got %q", captured.Profile)
	}
}

func TestServerProfileCommandIsRegistered(t *testing.T) {
	cmd := NewCommand(func() config.Config { return config.Config{} })
	found, _, err := cmd.Find([]string{"profile", "trojan-tls"})
	if err != nil {
		t.Fatalf("find profile command: %v", err)
	}
	if found == nil || found.Use != "profile [trojan-tls|vless-tls-vision]" {
		t.Fatalf("unexpected command: %#v", found)
	}
}
