package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestHasServerConfig(t *testing.T) {
	cases := []struct {
		name  string
		setup func(root, desiredConfigDir, liveConfigDir, pendingConfigDir string) error
		want  bool
	}{
		{
			name: "no config",
			setup: func(root, desiredConfigDir, liveConfigDir, pendingConfigDir string) error {
				return nil
			},
			want: false,
		},
		{
			name: "live config file",
			setup: func(root, desiredConfigDir, liveConfigDir, pendingConfigDir string) error {
				path := config.LiveConfigPath(layout.ServerConfigFileName)
				return writeTestFile(path, "server = {}\n")
			},
			want: true,
		},
		{
			name: "pending config file",
			setup: func(root, desiredConfigDir, liveConfigDir, pendingConfigDir string) error {
				path := config.PendingConfigPath(layout.ServerConfigFileName)
				return writeTestFile(path, "server = {}\n")
			},
			want: true,
		},
		{
			name: "live config files",
			setup: func(root, desiredConfigDir, liveConfigDir, pendingConfigDir string) error {
				return writeConfigFiles(liveConfigDir)
			},
			want: true,
		},
		{
			name: "pending config files",
			setup: func(root, desiredConfigDir, liveConfigDir, pendingConfigDir string) error {
				return writeConfigFiles(pendingConfigDir)
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			desiredConfigDir := filepath.Join(root, "config-server")
			liveConfigDir, err := config.LiveConfigDir(desiredConfigDir)
			if err != nil {
				t.Fatalf("live config dir: %v", err)
			}
			pendingConfigDir, err := config.PendingConfigDir(desiredConfigDir)
			if err != nil {
				t.Fatalf("pending config dir: %v", err)
			}
			if err := tc.setup(root, desiredConfigDir, liveConfigDir, pendingConfigDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			got, err := hasServerConfig(liveConfigDir, pendingConfigDir)
			if err != nil {
				t.Fatalf("hasServerConfig error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("hasServerConfig=%v, want %v", got, tc.want)
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
