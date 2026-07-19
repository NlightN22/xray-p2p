package config

import (
	"path/filepath"
	"testing"
)

func TestSchemaCompatibilityV026BaseConfig(t *testing.T) {
	for _, name := range []string{"xp2p-client.toml", "xp2p-server.toml"} {
		path := filepath.Join("..", "..", "..", "tests", "schema", "compat", "v0.2.6", name)
		cfg, err := Load(Options{Path: path})
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		if name == "xp2p-client.toml" && cfg.Client.ServerAddress != "server.example.com" {
			t.Fatalf("client server address = %q", cfg.Client.ServerAddress)
		}
		if name == "xp2p-server.toml" && cfg.Server.Host != "server.example.com" {
			t.Fatalf("server host = %q", cfg.Server.Host)
		}
	}
}
