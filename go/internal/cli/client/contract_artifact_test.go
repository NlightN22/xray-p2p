package clientcmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

type inactiveContractController struct{}

func (inactiveContractController) Start(context.Context, servicecontrol.Role) error { return nil }
func (inactiveContractController) Stop(context.Context, servicecontrol.Role) error  { return nil }
func (inactiveContractController) Status(context.Context, servicecontrol.Role) (servicecontrol.Status, error) {
	return servicecontrol.Status{State: "inactive"}, nil
}

func TestStage4ClientInstallPublishesTypedResult(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	cfg := clientCfg(filepath.Join(root, "install"), layout.ClientConfigDir)
	cfg.Client.TunEnabled = false
	restore := stubClientInstall(func(_ context.Context, opts client.InstallOptions) error {
		if opts.Password != "123e4567-e89b-12d3-a456-426614174000" {
			t.Fatalf("credential was not passed to the use case")
		}
		return nil
	})
	defer restore()

	ctx, result := clioutput.CaptureResult(context.Background())
	cmd := newClientInstallCmd(func() config.Config { return cfg })
	cmd.SetArgs([]string{
		"--host", "edge.example", "--port", "443", "--user", "unicode-\u96ea",
		"--password", "123e4567-e89b-12d3-a456-426614174000", "--mode", "proxy",
	})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}
	got, ok := result().(struct {
		Status     string `json:"status"`
		InstallDir string `json:"install_dir"`
		ConfigDir  string `json:"config_dir"`
		Host       string `json:"host"`
		Port       string `json:"port"`
		User       string `json:"user"`
	})
	if !ok || got.Status != "completed" || got.Host != "edge.example" ||
		got.Port != "443" || got.User != "unicode-\u96ea" {
		t.Fatalf("unexpected typed install result: %#v", result())
	}

	restore()
	restore = stubClientInstall(func(context.Context, client.InstallOptions) error {
		return errors.New("install rejected")
	})
	defer restore()
	cmd = newClientInstallCmd(func() config.Config { return cfg })
	cmd.SetArgs([]string{
		"--host", "edge.example", "--user", "unicode-\u96ea",
		"--password", "123e4567-e89b-12d3-a456-426614174000", "--mode", "proxy",
	})
	failureCtx, failureResult := clioutput.CaptureResult(context.Background())
	if err := cmd.ExecuteContext(failureCtx); err == nil || failureResult() != nil {
		t.Fatalf("install failure published a result: err=%v result=%#v", err, failureResult())
	}
}

func TestStage4ClientDeployPublishesTypedCredentialArtifact(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	cfg := clientCfg(filepath.Join(root, "install"), layout.ClientConfigDir)
	cfg.Client.TunEnabled = false
	cfg.Client.SocksAddress = ""
	restoreController := servicecontrol.SetDefaultForTesting(inactiveContractController{})
	defer restoreController()

	previousHandshake := performDeployHandshakeFunc
	secret := "123e4567-e89b-12d3-a456-426614174001"
	link := "trojan://" + secret + "@edge.example:443?security=tls&sni=edge.example#unicode-%E9%9B%AA"
	performDeployHandshakeFunc = func(context.Context, deployOptions) (deployResult, deployCompletionFunc, error) {
		return deployResult{Link: link}, nil, nil
	}
	defer func() { performDeployHandshakeFunc = previousHandshake }()
	restoreInstall := stubClientInstall(func(_ context.Context, _ client.InstallOptions) error {
		content := `[client]
tun_enabled = false
[[client.endpoints]]
hostname = "edge.example"
port = 443
user = "unicode-\u96ea"
password = "` + secret + `"
server_name = "edge.example"
`
		return os.WriteFile(filepath.Join(root, layout.ClientConfigFileName), []byte(content), 0o600)
	})
	defer restoreInstall()

	ctx, result := clioutput.CaptureResult(context.Background())
	cmd := newClientDeployCmd(func() config.Config { return cfg })
	cmd.SetArgs([]string{
		"--host", "deploy.example", "--user", "unicode-\u96ea",
		"--password", secret, "--mode", "proxy",
	})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}
	got, ok := result().(struct {
		Status      string `json:"status"`
		Link        string `json:"link"`
		InstallDir  string `json:"install_dir"`
		ConfigDir   string `json:"config_dir"`
		TunEnabled  bool   `json:"tun_enabled"`
		TunMode     string `json:"tun_mode"`
		ServiceLive bool   `json:"service_active"`
	})
	if !ok || got.Status != "completed" || got.Link != link || got.TunEnabled || got.ServiceLive {
		t.Fatalf("unexpected typed deploy result: %#v", result())
	}

	performDeployHandshakeFunc = func(context.Context, deployOptions) (deployResult, deployCompletionFunc, error) {
		return deployResult{}, nil, serverDeployError{msg: "password=" + secret}
	}
	failureCtx, failureResult := clioutput.CaptureResult(context.Background())
	cmd = newClientDeployCmd(func() config.Config { return cfg })
	cmd.SetArgs([]string{
		"--host", "deploy.example", "--user", "unicode-\u96ea",
		"--password", secret, "--mode", "proxy",
	})
	if err := cmd.ExecuteContext(failureCtx); err == nil || failureResult() != nil {
		t.Fatalf("deploy failure published a result: err=%v result=%#v", err, failureResult())
	}
}
