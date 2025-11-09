package root

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRuntimeDefaults(t *testing.T) {
	chdirTemp(t)
	opts := &rootOptions{}
	if err := opts.ensureRuntime(context.Background()); err != nil {
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
		logLevel:         "DEBUG",
		serverPort:       "65010",
		serverInstallDir: `D:\xp2p`,
		serverConfigDir:  "cfg-run",
		serverMode:       "MANUAL",
		serverCert:       `D:\certs\cert.pem`,
		serverKey:        `D:\certs\cert.key`,
		logJSON:          true,
	}
	if err := opts.ensureRuntime(context.Background()); err != nil {
		t.Fatalf("ensureRuntime failed: %v", err)
	}

	cfg := opts.cfg
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected debug level, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected logging format json, got %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "65010" {
		t.Fatalf("expected port 65010, got %s", cfg.Server.Port)
	}
	if cfg.Server.InstallDir != `D:\xp2p` {
		t.Fatalf("expected install dir D:\\xp2p, got %s", cfg.Server.InstallDir)
	}
	if cfg.Server.ConfigDir != "cfg-run" {
		t.Fatalf("expected config dir cfg-run, got %s", cfg.Server.ConfigDir)
	}
	if cfg.Server.Mode != "manual" {
		t.Fatalf("expected mode manual, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateFile != `D:\certs\cert.pem` {
		t.Fatalf("expected certificate D:\\certs\\cert.pem, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != `D:\certs\cert.key` {
		t.Fatalf("expected key D:\\certs\\cert.key, got %s", cfg.Server.KeyFile)
	}
}

func TestEnsureRuntimeWithConfigFile(t *testing.T) {
	chdirTemp(t)
	cfgPath := filepath.Join(".", "xp2p.yaml")
	writeFile(t, cfgPath, `
logging:
  level: warn
  format: json
server:
  port: 65011
  install_dir: C:\xp2p
  config_dir: cfg-config
  mode: manual
  certificate: C:\certs\server.pem
  key: C:\certs\server.key
`)

	opts := &rootOptions{configPath: cfgPath}
	if err := opts.ensureRuntime(context.Background()); err != nil {
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
	if cfg.Server.CertificateFile != `C:\certs\server.pem` {
		t.Fatalf("expected cert C:\\certs\\server.pem, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != `C:\certs\server.key` {
		t.Fatalf("expected key C:\\certs\\server.key, got %s", cfg.Server.KeyFile)
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
