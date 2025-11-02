package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	chdirTemp(t)

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
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
		t.Fatalf("expected mode auto, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateFile != "" {
		t.Fatalf("expected empty certificate path, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != "" {
		t.Fatalf("expected empty key path, got %s", cfg.Server.KeyFile)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := chdirTemp(t)

	writeFile(t, filepath.Join(dir, "xp2p.yaml"), `
logging:
  level: warn
  format: json
server:
  port: 65001
  install_dir: C:\xp2p-test
  config_dir: cfg-test
  mode: manual
  certificate: C:\certs\server.pem
  key: C:\certs\server.key
`)

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("expected warn level, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected json format, got %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "65001" {
		t.Fatalf("expected port 65001, got %s", cfg.Server.Port)
	}
	if cfg.Server.InstallDir != `C:\xp2p-test` {
		t.Fatalf("expected install dir C:\\xp2p-test, got %s", cfg.Server.InstallDir)
	}
	if cfg.Server.ConfigDir != "cfg-test" {
		t.Fatalf("expected config dir cfg-test, got %s", cfg.Server.ConfigDir)
	}
	if cfg.Server.Mode != "manual" {
		t.Fatalf("expected mode manual, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateFile != `C:\certs\server.pem` {
		t.Fatalf("expected certificate C:\\certs\\server.pem, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != `C:\certs\server.key` {
		t.Fatalf("expected key C:\\certs\\server.key, got %s", cfg.Server.KeyFile)
	}
}

func TestLoadFromEnv(t *testing.T) {
	chdirTemp(t)

	t.Setenv("XP2P_LOGGING_LEVEL", "DEBUG")
	t.Setenv("XP2P_LOGGING_FORMAT", "JSON")
	t.Setenv("XP2P_SERVER_PORT", "65002")
	t.Setenv("XP2P_SERVER_INSTALL_DIR", `D:\xp2p`)
	t.Setenv("XP2P_SERVER_CONFIG_DIR", "cfg-dir")
	t.Setenv("XP2P_SERVER_MODE", "AUTO")
	t.Setenv("XP2P_SERVER_CERTIFICATE", `D:\certs\cert.pem`)
	t.Setenv("XP2P_SERVER_KEY", `D:\certs\cert.key`)

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected debug level, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected json format, got %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "65002" {
		t.Fatalf("expected port 65002, got %s", cfg.Server.Port)
	}
	if cfg.Server.InstallDir != `D:\xp2p` {
		t.Fatalf("expected install dir D:\\xp2p, got %s", cfg.Server.InstallDir)
	}
	if cfg.Server.ConfigDir != "cfg-dir" {
		t.Fatalf("expected config dir cfg-dir, got %s", cfg.Server.ConfigDir)
	}
	if cfg.Server.Mode != "auto" {
		t.Fatalf("expected normalized mode auto, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateFile != `D:\certs\cert.pem` {
		t.Fatalf("expected certificate D:\\certs\\cert.pem, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != `D:\certs\cert.key` {
		t.Fatalf("expected key D:\\certs\\cert.key, got %s", cfg.Server.KeyFile)
	}
}

func TestLoadOverrides(t *testing.T) {
	chdirTemp(t)

	t.Setenv("XP2P_LOGGING_LEVEL", "debug")

	cfg, err := Load(Options{
		Overrides: map[string]any{
			"logging.level":      "error",
			"logging.format":     "json",
			"server.port":        "65003",
			"server.install_dir": `E:\xp2p`,
			"server.config_dir":  "cfg-override",
			"server.mode":        "MANUAL",
			"server.certificate": `E:\certs\cert.pem`,
			"server.key":         `E:\certs\cert.key`,
		},
	})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "error" {
		t.Fatalf("expected error level, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected json format, got %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "65003" {
		t.Fatalf("expected port 65003, got %s", cfg.Server.Port)
	}
	if cfg.Server.InstallDir != `E:\xp2p` {
		t.Fatalf("expected install dir E:\\xp2p, got %s", cfg.Server.InstallDir)
	}
	if cfg.Server.ConfigDir != "cfg-override" {
		t.Fatalf("expected config dir cfg-override, got %s", cfg.Server.ConfigDir)
	}
	if cfg.Server.Mode != "manual" {
		t.Fatalf("expected mode manual, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateFile != `E:\certs\cert.pem` {
		t.Fatalf("expected certificate E:\\certs\\cert.pem, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != `E:\certs\cert.key` {
		t.Fatalf("expected key E:\\certs\\cert.key, got %s", cfg.Server.KeyFile)
	}
}

func TestLoadWithExplicitPath(t *testing.T) {
	dir := chdirTemp(t)

	cfgPath := filepath.Join(dir, "custom.toml")
	writeFile(t, cfgPath, `
[logging]
level = "warn"
format = "json"

[server]
port = "65004"
install_dir = "C:\\xp2p-custom"
config_dir = "config-alt"
mode = "Manual"
certificate = "C:\\certs\\server.pem"
key = "C:\\certs\\server.key"
`)

	cfg, err := Load(Options{Path: cfgPath})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("expected warn level, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected json format, got %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "65004" {
		t.Fatalf("expected port 65004, got %s", cfg.Server.Port)
	}
	if cfg.Server.InstallDir != `C:\xp2p-custom` {
		t.Fatalf("expected install dir C:\\xp2p-custom, got %s", cfg.Server.InstallDir)
	}
	if cfg.Server.ConfigDir != "config-alt" {
		t.Fatalf("expected config dir config-alt, got %s", cfg.Server.ConfigDir)
	}
	if cfg.Server.Mode != "manual" {
		t.Fatalf("expected mode manual, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateFile != `C:\certs\server.pem` {
		t.Fatalf("expected certificate C:\\certs\\server.pem, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != `C:\certs\server.key` {
		t.Fatalf("expected key C:\\certs\\server.key, got %s", cfg.Server.KeyFile)
	}
}

func TestLoadInvalidPath(t *testing.T) {
	dir := chdirTemp(t)

	_, err := Load(Options{Path: dir})
	if err == nil {
		t.Fatalf("expected error for directory path")
	}
}

func chdirTemp(t *testing.T) string {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	return tmp
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
