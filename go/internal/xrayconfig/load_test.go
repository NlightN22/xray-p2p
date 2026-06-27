package xrayconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClientConfigUsesXrayLogSettings(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "xp2p-client.toml")
	writeXrayConfig(t, configPath, `
[client]
  [client.xray]
    [client.xray.logs]
      access = "/var/log/xp2p/client/access.log"
      level = "debug"
      stats_enabled = false
`)

	cfg, err := LoadClientConfigWithDefaults(configPath)
	if err != nil {
		t.Fatalf("load client xray config: %v", err)
	}
	assertLogSettings(t, cfg.Logs, "/var/log/xp2p/client/access.log")
}

func TestLoadServerConfigUsesXrayLogSettings(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "xp2p-server.toml")
	writeXrayConfig(t, configPath, `
[server]
  [server.xray]
    [server.xray.logs]
      access = "/var/log/xp2p/server/access.log"
      level = "debug"
      stats_enabled = false
`)

	cfg, err := LoadServerConfigWithDefaults(configPath)
	if err != nil {
		t.Fatalf("load server xray config: %v", err)
	}
	assertLogSettings(t, cfg.Logs, "/var/log/xp2p/server/access.log")
}

func writeXrayConfig(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func assertLogSettings(t *testing.T, logs LogsConfig, access string) {
	t.Helper()
	if logs.Level != "debug" || logs.Access != access {
		t.Fatalf("unexpected logs: %+v", logs)
	}
	if logs.StatsEnabled == nil || *logs.StatsEnabled {
		t.Fatalf("stats should be disabled: %+v", logs.StatsEnabled)
	}
}
