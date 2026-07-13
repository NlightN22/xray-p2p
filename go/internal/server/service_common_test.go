package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
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
			name: "apply request",
			setup: func(root, desiredConfigDir, liveConfigDir, pendingConfigDir string) error {
				return writeTestFile(config.ApplyRequestPath(), "{}\n")
			},
			want: true,
		},
		{
			name: "live artifacts",
			setup: func(root, desiredConfigDir, liveConfigDir, pendingConfigDir string) error {
				return writeConfigFiles(liveConfigDir)
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

			got, err := hasServerConfig(liveConfigDir)
			if err != nil {
				t.Fatalf("hasServerConfig error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("hasServerConfig=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestWriteApplyErrorForExistingRequest(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	req := apply.Request{ID: "req-1", Role: apply.RoleServer}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, ""); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	writeApplyErrorForExistingRequest(apply.RoleServer, os.ErrInvalid)

	marker, exists, err := apply.ReadError(config.ApplyErrorPath())
	if err != nil {
		t.Fatalf("ReadError: %v", err)
	}
	if !exists {
		t.Fatal("expected apply.error to be written")
	}
	if marker.RequestID != req.ID || marker.Role != apply.RoleServer {
		t.Fatalf("unexpected marker: %+v", marker)
	}
	if marker.Reason == "" {
		t.Fatal("expected marker reason")
	}
}

func TestWriteApplyErrorForEarlyFailureCreatesRequest(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)

	writeApplyErrorForExistingRequest(apply.RoleServer, os.ErrInvalid)

	req, exists, err := apply.ReadRequest(config.ApplyRequestPath())
	if err != nil {
		t.Fatalf("ReadRequest: %v", err)
	}
	if !exists || !req.MatchesRole(apply.RoleServer) || req.ID == "" {
		t.Fatalf("unexpected request: exists=%v request=%+v", exists, req)
	}
	marker, exists, err := apply.ReadError(config.ApplyErrorPath())
	if err != nil {
		t.Fatalf("ReadError: %v", err)
	}
	if !exists {
		t.Fatal("expected apply.error to be written")
	}
	if marker.RequestID != req.ID || marker.Role != apply.RoleServer {
		t.Fatalf("unexpected marker: %+v", marker)
	}
}

func TestRecordServerBootstrapApplyErrorWritesMarker(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)

	recordServerBootstrapApplyError(os.ErrInvalid)

	marker, exists, err := apply.ReadError(config.ApplyErrorPath())
	if err != nil {
		t.Fatalf("ReadError: %v", err)
	}
	if !exists {
		t.Fatal("expected apply.error to be written")
	}
	if marker.Role != apply.RoleServer || marker.RequestID == "" || marker.Reason == "" {
		t.Fatalf("unexpected marker: %+v", marker)
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
