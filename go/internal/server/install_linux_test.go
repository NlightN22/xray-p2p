//go:build linux

package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestNormalizeInstallOptionsLinux(t *testing.T) {
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")
	if err := os.WriteFile(certPath, []byte("CERT"), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("KEY"), 0o644); err != nil {
		t.Fatalf("write key: %v", err)
	}

	opts := InstallOptions{
		InstallDir:      tempDir,
		ConfigDir:       "",
		Port:            "62050",
		CertificateFile: certPath,
		KeyFile:         keyPath,
		Host:            "vpn.example",
	}

	state, err := normalizeInstallOptions(opts)
	if err != nil {
		t.Fatalf("normalizeInstallOptions: %v", err)
	}

	expectedConfig := filepath.Join(tempDir, layout.ServerConfigDir)
	if state.configDir != expectedConfig {
		t.Fatalf("configDir mismatch: got %s want %s", state.configDir, expectedConfig)
	}

	expectedLogs := filepath.Join(layout.UnixLogRoot, "server")
	if state.logsDir != expectedLogs {
		t.Fatalf("logsDir mismatch: got %s want %s", state.logsDir, expectedLogs)
	}

	expectedState := filepath.Join(tempDir, installstate.FileNameForKind(installstate.KindServer))
	if state.stateFile != expectedState {
		t.Fatalf("stateFile mismatch: got %s want %s", state.stateFile, expectedState)
	}
}
