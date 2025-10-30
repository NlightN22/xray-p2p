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
	if cfg.Server.Port != "62022" {
		t.Fatalf("unexpected server port: %s", cfg.Server.Port)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := chdirTemp(t)

	writeFile(t, filepath.Join(dir, "xp2p.yaml"), `
logging:
  level: warn
server:
  port: 65001
`)

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("expected warn level, got %s", cfg.Logging.Level)
	}
	if cfg.Server.Port != "65001" {
		t.Fatalf("expected port 65001, got %s", cfg.Server.Port)
	}
}

func TestLoadFromEnv(t *testing.T) {
	chdirTemp(t)

	t.Setenv("XP2P_LOGGING_LEVEL", "DEBUG")
	t.Setenv("XP2P_SERVER_PORT", "65002")

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected debug level, got %s", cfg.Logging.Level)
	}
	if cfg.Server.Port != "65002" {
		t.Fatalf("expected port 65002, got %s", cfg.Server.Port)
	}
}

func TestLoadOverrides(t *testing.T) {
	chdirTemp(t)

	t.Setenv("XP2P_LOGGING_LEVEL", "debug")

	cfg, err := Load(Options{
		Overrides: map[string]any{
			"logging.level": "error",
			"server.port":   "65003",
		},
	})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "error" {
		t.Fatalf("expected error level, got %s", cfg.Logging.Level)
	}
	if cfg.Server.Port != "65003" {
		t.Fatalf("expected port 65003, got %s", cfg.Server.Port)
	}
}

func TestLoadWithExplicitPath(t *testing.T) {
	dir := chdirTemp(t)

	cfgPath := filepath.Join(dir, "custom.toml")
	writeFile(t, cfgPath, `
[logging]
level = "warn"

[server]
port = "65004"
`)

	cfg, err := Load(Options{Path: cfgPath})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("expected warn level, got %s", cfg.Logging.Level)
	}
	if cfg.Server.Port != "65004" {
		t.Fatalf("expected port 65004, got %s", cfg.Server.Port)
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
