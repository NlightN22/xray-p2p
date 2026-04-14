package client

import (
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestApplyClientEndpointConfigDoesNotInheritAllowInsecure(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", baseDir)
	liveConfigDir := filepath.Join(baseDir, "config-client")
	configDir, err := config.PendingConfigDir(liveConfigDir)
	if err != nil {
		t.Fatalf("pending config dir: %v", err)
	}
	configPath := filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName))

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
	if err := state.save(configPath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	if _, err := applyClientEndpointConfig(configDir, configPath, endpointConfig{
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

	updated, err := loadClientInstallState(configPath)
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
