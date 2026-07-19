package xrayconfig

import (
	"path/filepath"
	"testing"
)

func TestSchemaCompatibilityV026XrayOverrides(t *testing.T) {
	root := filepath.Join("..", "..", "..", "tests", "schema", "compat", "v0.2.6")
	client, err := LoadClientConfigWithDefaults(filepath.Join(root, "xp2p-client.toml"))
	if err != nil {
		t.Fatalf("load v0.2.6 client Xray config: %v", err)
	}
	server, err := LoadServerConfigWithDefaults(filepath.Join(root, "xp2p-server.toml"))
	if err != nil {
		t.Fatalf("load v0.2.6 server Xray config: %v", err)
	}
	if client.Logs.Level != "warning" || server.Logs.Level != "warning" {
		t.Fatalf("legacy Xray log levels were not preserved: client=%q server=%q", client.Logs.Level, server.Logs.Level)
	}
}
