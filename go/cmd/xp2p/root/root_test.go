package root

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestEnsureRuntimeDefaults(t *testing.T) {
	chdirTemp(t)
	opts := &rootOptions{}
	if err := opts.ensureRuntime(newRootCmd()); err != nil {
		t.Fatalf("ensureRuntime failed: %v", err)
	}

	cfg := opts.cfg
	if cfg.Logging.Level != "info" {
		t.Fatalf("unexpected logging level: %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Fatalf("unexpected logging format: %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "62022" {
		t.Fatalf("unexpected server port: %s", cfg.Server.Port)
	}
	if cfg.Server.InstallDir == "" {
		t.Fatalf("expected non-empty install dir")
	}
	if cfg.Server.ConfigDir != "config-server" {
		t.Fatalf("expected default config dir config-server, got %s", cfg.Server.ConfigDir)
	}
	if cfg.Server.Mode != "auto" {
		t.Fatalf("expected default mode auto, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateStore != "" {
		t.Fatalf("expected empty certificate store, got %s", cfg.Server.CertificateStore)
	}
	if cfg.Server.CertificateFile != "" {
		t.Fatalf("expected empty certificate path, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != "" {
		t.Fatalf("expected empty key path, got %s", cfg.Server.KeyFile)
	}
}

func TestEnsureRuntimeWithOverrides(t *testing.T) {
	chdirTemp(t)
	opts := &rootOptions{
		logLevel: "DEBUG",
		logJSON:  true,
	}
	if err := opts.ensureRuntime(newRootCmd()); err != nil {
		t.Fatalf("ensureRuntime failed: %v", err)
	}

	cfg := opts.cfg
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected debug level, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected logging format json, got %s", cfg.Logging.Format)
	}
}

func TestEnsureRuntimeWithConfigFile(t *testing.T) {
	chdirTemp(t)
	cfgPath := filepath.Join(".", "xp2p-server.toml")
	writeFile(t, cfgPath, `
[logging]
level = "warn"
format = "json"

[server]
port = "65011"
install_dir = "C:\\xp2p"
config_dir = "cfg-config"
mode = "manual"
certificate = "C:\\certs\\server.pem"
key = "C:\\certs\\server.key"
`)

	opts := &rootOptions{configPath: cfgPath}
	if err := opts.ensureRuntime(newRootCmd()); err != nil {
		t.Fatalf("ensureRuntime failed: %v", err)
	}

	cfg := opts.cfg
	if cfg.Logging.Level != "warn" {
		t.Fatalf("expected warn level, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected logging format json, got %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "65011" {
		t.Fatalf("expected port 65011, got %s", cfg.Server.Port)
	}
	if cfg.Server.InstallDir != `C:\xp2p` {
		t.Fatalf("expected install dir C:\\xp2p, got %s", cfg.Server.InstallDir)
	}
	if cfg.Server.ConfigDir != "cfg-config" {
		t.Fatalf("expected config dir cfg-config, got %s", cfg.Server.ConfigDir)
	}
	if cfg.Server.Mode != "manual" {
		t.Fatalf("expected mode manual, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateStore != "" {
		t.Fatalf("expected empty certificate store, got %s", cfg.Server.CertificateStore)
	}
	if cfg.Server.CertificateFile != `C:\certs\server.pem` {
		t.Fatalf("expected cert C:\\certs\\server.pem, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != `C:\certs\server.key` {
		t.Fatalf("expected key C:\\certs\\server.key, got %s", cfg.Server.KeyFile)
	}
}

func TestServiceRunIgnoresInvalidConfig(t *testing.T) {
	cases := []struct {
		name string
		path []string
		want bool
	}{
		{name: "client service run", path: []string{"client", "service", "run"}, want: true},
		{name: "server service run", path: []string{"server", "service", "run"}, want: true},
		{name: "server run", path: []string{"server", "run"}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := commandPath(tc.path...)
			if got := shouldIgnoreInvalidConfig(cmd); got != tc.want {
				t.Fatalf("shouldIgnoreInvalidConfig(%q)=%v, want %v", cmd.CommandPath(), got, tc.want)
			}
		})
	}
}

func TestHeartbeatContractSkipsRuntimeConfig(t *testing.T) {
	cmd := commandPath("heartbeat", "contract")
	if !shouldSkipRuntime(cmd) {
		t.Fatal("heartbeat contract must not require runtime configuration")
	}
}

func chdirTemp(t *testing.T) {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Setenv("XP2P_CONFIG_ROOT", tmp)
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newRootCmd() *cobra.Command {
	return &cobra.Command{Use: "xp2p"}
}

func commandPath(names ...string) *cobra.Command {
	root := newRootCmd()
	parent := root
	for _, name := range names {
		child := &cobra.Command{Use: name}
		parent.AddCommand(child)
		parent = child
	}
	return parent
}
