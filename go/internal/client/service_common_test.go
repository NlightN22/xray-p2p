package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestHasClientConfig(t *testing.T) {
	cases := []struct {
		name  string
		setup func(root, configDir string) error
		want  bool
	}{
		{
			name: "no config",
			setup: func(root, configDir string) error {
				return nil
			},
			want: false,
		},
		{
			name: "live config file",
			setup: func(root, configDir string) error {
				path := config.ConfigPath(layout.ClientConfigFileName)
				return writeTestFile(path, "client = {}\n")
			},
			want: true,
		},
		{
			name: "pending config file",
			setup: func(root, configDir string) error {
				path := config.PendingConfigPath(layout.ClientConfigFileName)
				return writeTestFile(path, "client = {}\n")
			},
			want: true,
		},
		{
			name: "live config files",
			setup: func(root, configDir string) error {
				return writeConfigFiles(configDir)
			},
			want: true,
		},
		{
			name: "pending config files",
			setup: func(root, configDir string) error {
				return writeConfigFiles(apply.PendingDir(configDir))
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			configDir := filepath.Join(root, "config-client")
			if err := tc.setup(root, configDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			got, err := hasClientConfig(configDir)
			if err != nil {
				t.Fatalf("hasClientConfig error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("hasClientConfig=%v, want %v", got, tc.want)
			}
		})
	}
}

func writeConfigFiles(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range runRequiredConfigFiles {
		if err := writeTestFile(filepath.Join(dir, name), "{}\n"); err != nil {
			return err
		}
	}
	return nil
}

func writeTestFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
