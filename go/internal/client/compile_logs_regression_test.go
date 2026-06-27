package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCompileDesiredUsesClientXrayLogSettings(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "xp2p-client.toml")
	extensionsDir := filepath.Join(dir, "extensions")
	if err := os.MkdirAll(extensionsDir, 0o755); err != nil {
		t.Fatalf("mkdir extensions: %v", err)
	}
	config := `
[client]
  endpoints = [{ address = "127.0.0.1", hostname = "127.0.0.1", password = "secret", port = 443, profile = "trojan-tls", protocol = "trojan", security = "tls", tag = "proxy-local", transport = "tcp", user = "alice" }]

  [client.xray]

    [client.xray.logs]
      access = "/var/log/xp2p/client/access.log"
      level = "debug"
      stats_enabled = false
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	artifacts, err := compileDesired(configPath, extensionsDir)
	if err != nil {
		t.Fatalf("compile desired: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(artifacts.XrayJSON, &doc); err != nil {
		t.Fatalf("parse xray: %v", err)
	}
	logDoc, _ := doc["log"].(map[string]any)
	if logDoc["loglevel"] != "debug" || logDoc["access"] != "/var/log/xp2p/client/access.log" {
		t.Fatalf("unexpected log doc: %#v", logDoc)
	}
	if _, ok := doc["stats"]; ok {
		t.Fatalf("stats should be disabled: %#v", doc["stats"])
	}
}
