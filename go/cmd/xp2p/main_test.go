package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestParseRootArgsDefaults(t *testing.T) {
	chdirTemp(t)

	cfg, rest, versionRequested, err := parseRootArgs(nil)
	if err != nil {
		t.Fatalf("parseRootArgs failed: %v", err)
	}
	if versionRequested {
		t.Fatalf("expected versionRequested to be false")
	}
	if len(rest) != 0 {
		t.Fatalf("expected no remaining args, got %v", rest)
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
		t.Fatalf("expected default mode auto, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateFile != "" {
		t.Fatalf("expected empty certificate path, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != "" {
		t.Fatalf("expected empty key path, got %s", cfg.Server.KeyFile)
	}
}

func TestParseRootArgsWithFlags(t *testing.T) {
	chdirTemp(t)

	args := []string{
		"--log-level", "DEBUG",
		"--server-port", "65010",
		"--server-install-dir", `D:\xp2p`,
		"--server-config-dir", "cfg-run",
		"--server-mode", "MANUAL",
		"--server-cert", `D:\certs\cert.pem`,
		"--server-key", `D:\certs\cert.key`,
		"--log-json",
		"ping", "--count", "3",
	}

	cfg, rest, versionRequested, err := parseRootArgs(args)
	if err != nil {
		t.Fatalf("parseRootArgs failed: %v", err)
	}
	if versionRequested {
		t.Fatalf("expected versionRequested to be false")
	}
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
	expected := []string{"ping", "--count", "3"}
	if len(rest) != len(expected) {
		t.Fatalf("unexpected remaining args: %v", rest)
	}
	for i, v := range expected {
		if rest[i] != v {
			t.Fatalf("expected rest[%d]=%s, got %s", i, v, rest[i])
		}
	}
}

func TestParseRootArgsWithConfigFile(t *testing.T) {
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

	cfg, rest, versionRequested, err := parseRootArgs([]string{"--config", cfgPath})
	if err != nil {
		t.Fatalf("parseRootArgs failed: %v", err)
	}
	if versionRequested {
		t.Fatalf("expected versionRequested to be false")
	}
	if len(rest) != 0 {
		t.Fatalf("expected no remaining args, got %v", rest)
	}
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

func TestParseRootArgsHelp(t *testing.T) {
	chdirTemp(t)

	_, _, _, err := parseRootArgs([]string{"--help"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestParseRootArgsVersionFlag(t *testing.T) {
	chdirTemp(t)

	_, rest, versionRequested, err := parseRootArgs([]string{"--version"})
	if err != nil {
		t.Fatalf("parseRootArgs failed: %v", err)
	}
	if !versionRequested {
		t.Fatalf("expected versionRequested to be true")
	}
	if len(rest) != 0 {
		t.Fatalf("expected no remaining args, got %v", rest)
	}
}

func TestParseRootArgsVersionWithArgsFails(t *testing.T) {
	chdirTemp(t)

	_, _, _, err := parseRootArgs([]string{"--version", "ping"})
	if err == nil {
		t.Fatal("expected error when combining --version with positional arguments")
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
