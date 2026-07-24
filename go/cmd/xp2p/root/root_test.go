package root

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
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

func TestEveryExecutableLeafHasOutputClassification(t *testing.T) {
	root := NewCommand()
	seen := make(map[string]bool)
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		children := cmd.Commands()
		if len(children) == 0 && (cmd.Run != nil || cmd.RunE != nil) {
			seen[cmd.CommandPath()] = true
			if class := clioutput.Class(cmd); class == "" {
				t.Errorf("%s has no output classification", cmd.CommandPath())
			}
			contract, ok := outputContractInventory[cmd.CommandPath()]
			if !ok {
				t.Errorf("%s is absent from the explicit output inventory", cmd.CommandPath())
			} else if contract.class != clioutput.ClassJSON && contract.reason == "" {
				t.Errorf("%s exception has no reason", cmd.CommandPath())
			}
		}
		for _, child := range children {
			visit(child)
		}
	}
	visit(root)
	for path := range outputContractInventory {
		if !seen[path] && !platformSpecificOutputContracts[path] {
			t.Errorf("stale output inventory entry %q", path)
		}
	}
}

func TestHeartbeatContractJSONEnvelope(t *testing.T) {
	root := NewCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--json", "heartbeat", "contract"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v; stderr=%s", err, stderr.String())
	}
	var envelope clioutput.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode stdout without cleanup: %v; stdout=%q", err, stdout.String())
	}
	if envelope.SchemaVersion != clioutput.SchemaVersion {
		t.Fatalf("schema version=%q", envelope.SchemaVersion)
	}
	if envelope.Command != "xp2p heartbeat contract" {
		t.Fatalf("command=%q", envelope.Command)
	}
}

func TestGeneratorRejectsJSONBeforeExecution(t *testing.T) {
	root := NewCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--json", "completion", "bash"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected unsupported output error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout must be empty, got %q", stdout.String())
	}
	var envelope clioutput.ErrorEnvelope
	if decodeErr := json.Unmarshal(stderr.Bytes(), &envelope); decodeErr != nil {
		t.Fatalf("decode stderr: %v; stderr=%q", decodeErr, stderr.String())
	}
	if envelope.Error.Code != "unsupported_output_format" {
		t.Fatalf("code=%q", envelope.Error.Code)
	}
}

func TestHelpInJSONModeIsOneDocument(t *testing.T) {
	root := NewCommand()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"--json", "client", "list", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var envelope clioutput.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode help: %v; stdout=%q", err, stdout.String())
	}
	if envelope.Command != "xp2p client list" {
		t.Fatalf("command=%q", envelope.Command)
	}
}

func TestArgumentErrorUsesJSONStderr(t *testing.T) {
	root := NewCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--json", "heartbeat", "contract", "extra"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected argument error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout=%q", stdout.String())
	}
	var envelope clioutput.ErrorEnvelope
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("decode stderr: %v; stderr=%q", err, stderr.String())
	}
	if envelope.Error.Code != "invalid_argument" {
		t.Fatalf("code=%q", envelope.Error.Code)
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
