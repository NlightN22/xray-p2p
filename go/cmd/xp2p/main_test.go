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

	cfg, rest, err := parseRootArgs(nil)
	if err != nil {
		t.Fatalf("parseRootArgs failed: %v", err)
	}
	if len(rest) != 0 {
		t.Fatalf("expected no remaining args, got %v", rest)
	}
	if cfg.Logging.Level != "info" {
		t.Fatalf("unexpected logging level: %s", cfg.Logging.Level)
	}
	if cfg.Server.Port != "62022" {
		t.Fatalf("unexpected server port: %s", cfg.Server.Port)
	}
}

func TestParseRootArgsWithFlags(t *testing.T) {
	chdirTemp(t)

	args := []string{"--log-level", "DEBUG", "--server-port", "65010", "ping", "--count", "3"}

	cfg, rest, err := parseRootArgs(args)
	if err != nil {
		t.Fatalf("parseRootArgs failed: %v", err)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected debug level, got %s", cfg.Logging.Level)
	}
	if cfg.Server.Port != "65010" {
		t.Fatalf("expected port 65010, got %s", cfg.Server.Port)
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
server:
  port: 65011
`)

	cfg, rest, err := parseRootArgs([]string{"--config", cfgPath})
	if err != nil {
		t.Fatalf("parseRootArgs failed: %v", err)
	}
	if len(rest) != 0 {
		t.Fatalf("expected no remaining args, got %v", rest)
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("expected warn level, got %s", cfg.Logging.Level)
	}
	if cfg.Server.Port != "65011" {
		t.Fatalf("expected port 65011, got %s", cfg.Server.Port)
	}
}

func TestParseRootArgsHelp(t *testing.T) {
	chdirTemp(t)

	_, _, err := parseRootArgs([]string{"--help"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
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
