package client

import (
	"path/filepath"
	"testing"
)

func TestApplyClientEndpointConfigDoesNotInheritAllowInsecure(t *testing.T) {
	baseDir := t.TempDir()
	configDir := filepath.Join(baseDir, "config-client")
	statePath := filepath.Join(baseDir, "install-state-client.json")

	state := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{
				Hostname:      "alpha.example",
				Tag:           buildProxyTag("alpha.example"),
				Address:       "alpha.example",
				Port:          443,
				User:          "alpha@example.com",
				Password:      "alpha-pass",
				ServerName:    "alpha.example",
				AllowInsecure: true,
			},
		},
	}
	if err := state.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if err := applyClientEndpointConfig(configDir, statePath, endpointConfig{
		Hostname:              "beta.example",
		Port:                  443,
		User:                  "beta@example.com",
		Password:              "beta-pass",
		ServerName:            "beta.example",
		AllowInsecure:         false,
		AllowInsecureOverride: false,
	}, false); err != nil {
		t.Fatalf("apply endpoint config: %v", err)
	}

	updated, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	for _, ep := range updated.Endpoints {
		if ep.Hostname == "beta.example" {
			if ep.AllowInsecure {
				t.Fatalf("expected allow insecure to remain false for beta.example")
			}
			return
		}
	}
	t.Fatalf("beta.example endpoint not found in state")
}
