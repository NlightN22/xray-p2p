package root

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	clientcmd "github.com/NlightN22/xray-p2p/go/internal/cli/client"
	servercmd "github.com/NlightN22/xray-p2p/go/internal/cli/server"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

type stage4InactiveController struct{}

func (stage4InactiveController) Start(context.Context, servicecontrol.Role) error { return nil }
func (stage4InactiveController) Stop(context.Context, servicecontrol.Role) error  { return nil }
func (stage4InactiveController) Status(context.Context, servicecontrol.Role) (servicecontrol.Status, error) {
	return servicecontrol.Status{State: "inactive"}, nil
}

func clientInstallStage4Contract() stage4Contract {
	return stage4Contract{
		success: func(t *testing.T, path string) {
			fixture := newStage4ClientFixture(t)
			restore := clientcmd.SetInstallForTesting(func(_ context.Context, opts client.InstallOptions) error {
				if opts.Password != fixture.secret {
					t.Fatalf("credential did not reach install boundary")
				}
				return nil
			})
			t.Cleanup(restore)
			execution := executeContractCase(fixture.installArgs(), false)
			result := assertStage4Success(t, path, execution)
			if result["status"] != "completed" || result["host"] != "127.0.0.1" ||
				result["port"] != "443" || result["user"] != fixture.user {
				t.Fatalf("unexpected client install result: %#v", result)
			}
			if !strings.Contains(execution.stdout, `unicode-雪\n\t\u0001`) {
				t.Fatalf("public JSON did not escape Unicode/control input: %q", execution.stdout)
			}
			assertNoCredentialFields(t, result, "result")
		},
		failure: func(t *testing.T, path string) {
			fixture := newStage4ClientFixture(t)
			restore := clientcmd.SetInstallForTesting(func(context.Context, client.InstallOptions) error {
				return errors.New("install rejected password=" + fixture.secret)
			})
			t.Cleanup(restore)
			execution := executeContractCase(fixture.installArgs(), false)
			assertStage4Failure(t, path, execution, fixture.secret)
		},
		human: func(t *testing.T, path string) {
			fixture := newStage4ClientFixture(t)
			restore := clientcmd.SetInstallForTesting(func(context.Context, client.InstallOptions) error {
				return nil
			})
			t.Cleanup(restore)
			stdout, stderr, err := executeHumanContractCase(fixture.installArgs())
			assertStage4Human(t, path, stdout, stderr, err, "client installed")
		},
	}
}

func clientDeployStage4Contract() stage4Contract {
	return stage4Contract{
		success: func(t *testing.T, path string) {
			fixture := newStage4ClientFixture(t)
			fixture.stubDeploySuccess(t)
			execution := executeContractCase(fixture.deployArgs(), false)
			result := assertStage4Success(t, path, execution)
			if result["status"] != "completed" || result["link"] != fixture.link ||
				result["tun_enabled"] != false || result["service_active"] != false {
				t.Fatalf("unexpected client deploy result: %#v", result)
			}
			link, ok := result["link"].(string)
			if !ok || !strings.Contains(link, "unicode-\u96ea%0A%09%01") {
				t.Fatalf("deploy result lost Unicode/control metadata: %#v", result["link"])
			}
			if !strings.Contains(execution.stdout, "unicode-\u96ea%0A%09%01") {
				t.Fatalf("public JSON lost Unicode/control metadata: %q", execution.stdout)
			}
		},
		failure: func(t *testing.T, path string) {
			fixture := newStage4ClientFixture(t)
			restore := clientcmd.SetDeployHandshakeForTesting(
				func(context.Context) (clientcmd.DeployHandshakeTestResult, error) {
					return clientcmd.DeployHandshakeTestResult{},
						errors.New("server rejected password=" + fixture.secret)
				},
			)
			t.Cleanup(restore)
			execution := executeContractCase(fixture.deployArgs(), false)
			assertStage4Failure(t, path, execution, fixture.secret)
		},
		human: func(t *testing.T, path string) {
			fixture := newStage4ClientFixture(t)
			fixture.stubDeploySuccess(t)
			stdout, stderr, err := executeHumanContractCase(fixture.deployArgs())
			assertStage4Human(t, path, stdout, stderr, err, "client deploy: completed")
		},
	}
}

type stage4ClientFixture struct {
	root       string
	installDir string
	secret     string
	user       string
	link       string
}

func newStage4ClientFixture(t *testing.T) stage4ClientFixture {
	t.Helper()
	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	secret := "123e4567-e89b-12d3-a456-426614174001"
	user := "unicode-\u96ea\n\t\x01"
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	content := "[client]\ninstall_dir = " + stage4Quote(installDir) + "\n" +
		"config_dir = \"config-client\"\ntun_enabled = false\nsocks_address = \"\"\n"
	if err := os.WriteFile(filepath.Join(root, layout.ClientConfigFileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	restoreController := servicecontrol.SetDefaultForTesting(stage4InactiveController{})
	t.Cleanup(restoreController)
	return stage4ClientFixture{
		root: root, installDir: installDir, secret: secret, user: user,
		link: "trojan://" + secret + "@edge.example:443?security=tls&sni=edge.example#unicode-\u96ea%0A%09%01",
	}
}

func (f stage4ClientFixture) installArgs() []string {
	return []string{
		"client", "install", "--path", f.installDir, "--config-dir", "config-client",
		"--host", "127.0.0.1", "--port", "443", "--user", f.user,
		"--password", f.secret, "--sni", "edge.example", "--mode", "proxy", "--force",
	}
}

func (f stage4ClientFixture) deployArgs() []string {
	return []string{
		"client", "deploy", "--host", "deploy.example", "--user", f.user,
		"--password", f.secret, "--mode", "proxy",
	}
}

func (f stage4ClientFixture) stubDeploySuccess(t *testing.T) {
	t.Helper()
	restoreHandshake := clientcmd.SetDeployHandshakeForTesting(
		func(context.Context) (clientcmd.DeployHandshakeTestResult, error) {
			return clientcmd.DeployHandshakeTestResult{Link: f.link}, nil
		},
	)
	t.Cleanup(restoreHandshake)
	restoreInstall := clientcmd.SetInstallForTesting(func(context.Context, client.InstallOptions) error {
		content := "[client]\ninstall_dir = " + stage4Quote(f.installDir) +
			"\nconfig_dir = \"config-client\"\ntun_enabled = false\n" +
			"[[client.endpoints]]\nhostname = \"edge.example\"\nport = 443\n" +
			"user = " + stage4Quote(f.user) + "\npassword = " + stage4Quote(f.secret) +
			"\nserver_name = \"edge.example\"\n"
		return os.WriteFile(filepath.Join(f.root, layout.ClientConfigFileName), []byte(content), 0o600)
	})
	t.Cleanup(restoreInstall)
	restoreStage := clientcmd.SetStageEndpointForTesting(func(context.Context, client.InstallOptions) error {
		return nil
	})
	t.Cleanup(restoreStage)
}

func serverInstallStage4Contract() stage4Contract {
	return stage4Contract{
		success: func(t *testing.T, path string) {
			fixture := newStage4ServerInstallFixture(t, false)
			generated := fixture.stubSuccess(t)
			result := assertStage4Success(t, path, executeContractCase(fixture.args(), false))
			credential, _ := result["credential"].(map[string]any)
			password, _ := credential["password"].(string)
			link, _ := credential["link"].(string)
			if result["status"] != "completed" || !tunnel.IsUUIDCredential(password) ||
				password != *generated || !strings.Contains(link, password) {
				t.Fatalf("unexpected server install result: %#v", result)
			}
		},
		failure: func(t *testing.T, path string) {
			fixture := newStage4ServerInstallFixture(t, false)
			secret := "server-install-secret"
			restore := servercmd.SetInstallForTesting(func(context.Context, server.InstallOptions) error {
				return errors.New("install rejected password=" + secret)
			})
			t.Cleanup(restore)
			execution := executeContractCase(fixture.args(), false)
			assertStage4Failure(t, path, execution, secret)
		},
		human: func(t *testing.T, path string) {
			fixture := newStage4ServerInstallFixture(t, false)
			fixture.stubSuccess(t)
			stdout, stderr, err := executeHumanContractCase(fixture.args())
			assertStage4Human(t, path, stdout, stderr, err, "server installed", "Generated server credential")
		},
	}
}

type stage4ServerInstallFixture struct {
	root       string
	installDir string
}

func newStage4ServerInstallFixture(t *testing.T, withCredential bool) stage4ServerInstallFixture {
	t.Helper()
	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	content := "[server]\ninstall_dir = " + stage4Quote(installDir) +
		"\nconfig_dir = \"config-server\"\nhost = \"edge.example\"\ntun_enabled = false\n"
	if withCredential {
		content += "\n[client]\nuser = \"existing\"\npassword = \"existing\"\n"
	}
	if err := os.WriteFile(filepath.Join(root, layout.ServerConfigFileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return stage4ServerInstallFixture{root: root, installDir: installDir}
}

func (f stage4ServerInstallFixture) args() []string {
	return []string{
		"server", "install", "--path", f.installDir, "--config-dir", "config-server",
		"--host", "edge.example", "--profile", "trojan-tls",
	}
}

func (f stage4ServerInstallFixture) stubSuccess(t *testing.T) *string {
	t.Helper()
	restoreInstall := servercmd.SetInstallForTesting(func(context.Context, server.InstallOptions) error {
		return nil
	})
	t.Cleanup(restoreInstall)
	generated := new(string)
	restoreAdd := servercmd.SetUserAddForTesting(func(_ context.Context, opts server.AddUserOptions) error {
		*generated = opts.Password
		return nil
	})
	t.Cleanup(restoreAdd)
	restoreLink := servercmd.SetUserLinkForTesting(
		func(_ context.Context, opts server.UserLinkOptions) (server.UserLink, error) {
			return server.UserLink{
				UserID: opts.UserID, Password: *generated,
				Link: "trojan://" + *generated + "@edge.example:58443?security=tls#generated",
			}, nil
		},
	)
	t.Cleanup(restoreLink)
	return generated
}
