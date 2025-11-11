package servercmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestServerCommandsAcceptDiagnosticsFlags(t *testing.T) {
	tempInstall := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		prepareInstallation(t, dir, layout.ServerConfigDir)
		return dir
	}

	baseCfg := func() config.Config {
		return config.Config{
			Server: config.ServerConfig{
				InstallDir: `C:\xp2p`,
				ConfigDir:  layout.ServerConfigDir,
				Host:       "srv.example.com",
				Port:       "62022",
			},
			Client: config.ClientConfig{
				User:     "user@example.com",
				Password: "secret",
			},
		}
	}

	cases := []struct {
		name string
		cfg  func(*testing.T) config.Config
		args []string
		stub func(*testing.T) func()
	}{
		{
			name: "install",
			cfg:  func(*testing.T) config.Config { return baseCfg() },
			args: []string{"install", "--path", `D:\xp2p`, "--config-dir", "cfg", "--host", "srv.internal", "--port", "65010", "--force"},
			stub: expectInstallCall,
		},
		{
			name: "remove",
			cfg:  func(*testing.T) config.Config { return baseCfg() },
			args: []string{"remove", "--path", `D:\xp2p`, "--config-dir", "cfg", "--keep-files", "--ignore-missing"},
			stub: expectRemoveCall,
		},
		{
			name: "run",
			cfg: func(t *testing.T) config.Config {
				dir := tempInstall(t)
				return config.Config{
					Server: config.ServerConfig{
						InstallDir: dir,
						ConfigDir:  layout.ServerConfigDir,
						Port:       "62022",
					},
				}
			},
			args: func() []string {
				// placeholder, replaced per test
				return nil
			}(),
			stub: expectRunCall,
		},
		{
			name: "user add",
			cfg:  func(*testing.T) config.Config { return baseCfg() },
			args: []string{"user", "add", "--path", `C:\xp2p`, "--config-dir", layout.ServerConfigDir, "--id", "user1", "--password", "pass1", "--host", "srv.example.com"},
			stub: expectUserAddCall,
		},
		{
			name: "user remove",
			cfg:  func(*testing.T) config.Config { return baseCfg() },
			args: []string{"user", "remove", "--path", `C:\xp2p`, "--config-dir", layout.ServerConfigDir, "--id", "user1"},
			stub: expectUserRemoveCall,
		},
		{
			name: "user list",
			cfg:  func(*testing.T) config.Config { return baseCfg() },
			args: []string{"user", "list", "--path", `C:\xp2p`, "--config-dir", layout.ServerConfigDir, "--host", "srv.example.com"},
			stub: expectUserListCall,
		},
		{
			name: "cert set",
			cfg:  func(*testing.T) config.Config { return baseCfg() },
			args: []string{"cert", "set", "--path", `C:\xp2p`, "--config-dir", layout.ServerConfigDir, "--cert", `C:\certs\cert.pem`, "--key", `C:\certs\cert.key`, "--host", "srv.example.com", "--force"},
			stub: expectCertSetCall,
		},
		{
			name: "deploy",
			cfg:  func(*testing.T) config.Config { return baseCfg() },
			args: []string{"deploy", "--listen", ":62090", "--link", "xp2p+deploy://host.example.com?cipher=AA&nonce=AA"},
			stub: expectDeployCall,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()

			cfg := tc.cfg(t)
			args := tc.args
			if tc.name == "run" {
				dir := cfg.Server.InstallDir
				args = []string{"run", "--path", dir, "--config-dir", cfg.Server.ConfigDir, "--xray-log-file", filepath.Join(dir, "logs", "server.err")}
			}

			var cleanup func()
			if tc.stub != nil {
				cleanup = tc.stub(t)
			}
			if cleanup != nil {
				t.Cleanup(cleanup)
			}

			cmd := NewCommand(func() config.Config { return cfg })
			root := newServerTestRoot(cmd)
			fullArgs := append([]string{"server"}, args...)
			fullArgs = append(fullArgs, "-P", "62080", "--diag-service-mode", "manual")
			root.SetArgs(fullArgs)

			if err := root.Execute(); err != nil {
				t.Fatalf("command execution failed: %v", err)
			}
		})
	}
}

func newServerTestRoot(cmd *cobra.Command) *cobra.Command {
	root := &cobra.Command{Use: "xp2p"}
	root.PersistentFlags().StringP("diag-service-port", "P", "", "")
	root.PersistentFlags().String("diag-service-mode", "", "")
	root.AddCommand(cmd)
	return root
}

func expectInstallCall(t *testing.T) func() {
	called := 0
	cleanup := stubServerInstall(func(context.Context, server.InstallOptions) error {
		called++
		return nil
	})
	return func() {
		cleanup()
		if called != 1 {
			t.Fatalf("serverInstallFunc called %d times", called)
		}
	}
}

func expectRemoveCall(t *testing.T) func() {
	called := 0
	cleanup := stubServerRemove(func(context.Context, server.RemoveOptions) error {
		called++
		return nil
	})
	return func() {
		cleanup()
		if called != 1 {
			t.Fatalf("serverRemoveFunc called %d times", called)
		}
	}
}

func expectRunCall(t *testing.T) func() {
	called := 0
	cleanup := stubServerRun(func(context.Context, server.RunOptions) error {
		called++
		return nil
	})
	return func() {
		cleanup()
		if called != 1 {
			t.Fatalf("serverRunFunc called %d times", called)
		}
	}
}

func expectUserAddCall(t *testing.T) func() {
	added := 0
	cleanupAdd := stubServerUserAdd(func(context.Context, server.AddUserOptions) error {
		added++
		return nil
	})
	cleanupLink := stubServerUserLink(func(context.Context, server.UserLinkOptions) (server.UserLink, error) {
		return server.UserLink{UserID: "user1", Link: "trojan://example"}, nil
	})
	return func() {
		cleanupLink()
		cleanupAdd()
		if added != 1 {
			t.Fatalf("serverUserAddFunc called %d times", added)
		}
	}
}

func expectUserRemoveCall(t *testing.T) func() {
	prev := serverUserRemoveFunc
	called := 0
	serverUserRemoveFunc = func(context.Context, server.RemoveUserOptions) error {
		called++
		return nil
	}
	return func() {
		serverUserRemoveFunc = prev
		if called != 1 {
			t.Fatalf("serverUserRemoveFunc called %d times", called)
		}
	}
}

func expectUserListCall(t *testing.T) func() {
	called := 0
	cleanup := stubServerUserList(func(context.Context, server.ListUsersOptions) ([]server.UserLink, error) {
		called++
		return []server.UserLink{}, nil
	})
	return func() {
		cleanup()
		if called != 1 {
			t.Fatalf("serverUserListFunc called %d times", called)
		}
	}
}

func expectCertSetCall(t *testing.T) func() {
	called := 0
	cleanup := stubServerSetCertificate(func(context.Context, server.CertificateOptions) error {
		called++
		return nil
	})
	return func() {
		cleanup()
		if called != 1 {
			t.Fatalf("serverSetCertFunc called %d times", called)
		}
	}
}

func expectDeployCall(t *testing.T) func() {
	prev := serverDeployFunc
	called := 0
	serverDeployFunc = func(context.Context, config.Config, serverDeployOptions) int {
		called++
		return 0
	}
	return func() {
		serverDeployFunc = prev
		if called != 1 {
			t.Fatalf("serverDeployFunc called %d times", called)
		}
	}
}
