package servercmd

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

func TestStage4ServerInstallPublishesGeneratedCredential(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	cfg := serverCfg(filepath.Join(root, "install"), layout.ServerConfigDir, "edge.example")
	cfg.Server.TunEnabled = false
	restoreInstall := stubServerInstall(func(context.Context, server.InstallOptions) error { return nil })
	defer restoreInstall()

	var generated string
	restoreAdd := stubServerUserAdd(func(_ context.Context, opts server.AddUserOptions) error {
		generated = opts.Password
		return nil
	})
	defer restoreAdd()
	restoreLink := stubServerUserLink(func(_ context.Context, opts server.UserLinkOptions) (server.UserLink, error) {
		return server.UserLink{
			UserID: opts.UserID, Password: generated,
			Link: "trojan://" + generated + "@edge.example:58443?security=tls#generated",
		}, nil
	})
	defer restoreLink()

	ctx, result := clioutput.CaptureResult(context.Background())
	cmd := newServerInstallCmd(func() config.Config { return cfg })
	cmd.SetArgs([]string{"--host", "edge.example", "--profile", "trojan-tls"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}
	if !tunnel.IsUUIDCredential(generated) {
		t.Fatalf("install did not generate a tunnel credential: %q", generated)
	}
	raw, err := json.Marshal(result())
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Status     string `json:"status"`
		InstallDir string `json:"install_dir"`
		ConfigDir  string `json:"config_dir"`
		Credential *struct {
			User     string  `json:"user"`
			Password string  `json:"password"`
			Link     *string `json:"link"`
		} `json:"credential"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "completed" || got.Credential == nil ||
		got.Credential.Password != generated || got.Credential.Link == nil ||
		!strings.Contains(*got.Credential.Link, generated) || got.Warnings == nil {
		t.Fatalf("unexpected server install result: %s", raw)
	}

	restoreInstall()
	restoreInstall = stubServerInstall(func(context.Context, server.InstallOptions) error {
		return errors.New("install failed with password=" + generated)
	})
	defer restoreInstall()
	failureCtx, failureResult := clioutput.CaptureResult(context.Background())
	cmd = newServerInstallCmd(func() config.Config { return cfg })
	cmd.SetArgs([]string{"--host", "edge.example", "--profile", "trojan-tls"})
	if err := cmd.ExecuteContext(failureCtx); err == nil || failureResult() != nil {
		t.Fatalf("server install failure published a result: err=%v result=%#v", err, failureResult())
	}
}
