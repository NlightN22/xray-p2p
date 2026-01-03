package server

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestBuildServerInstallBaseDefaults(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config-server")

	base, err := buildServerInstallBase(installDir, configDir, InstallOptions{
		InstallDir: installDir,
		ConfigDir:  "config-server",
		Host:       "10.0.0.1",
	})
	if err != nil {
		t.Fatalf("buildServerInstallBase error: %v", err)
	}
	if base.portStr != strconv.Itoa(DefaultTrojanPort) {
		t.Fatalf("portStr = %s", base.portStr)
	}
	if base.portVal != DefaultTrojanPort {
		t.Fatalf("portVal = %d", base.portVal)
	}
	if !base.selfSigned {
		t.Fatalf("expected selfSigned to be true")
	}
	if base.installOpts.Host != "10.0.0.1" {
		t.Fatalf("installOpts host = %s", base.installOpts.Host)
	}
	if base.installOpts.ConfigDir != "config-server" {
		t.Fatalf("installOpts configDir = %s", base.installOpts.ConfigDir)
	}
}

func TestBuildServerInstallBaseRejectsKeyWithoutCert(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config-server")

	_, err := buildServerInstallBase(installDir, configDir, InstallOptions{
		InstallDir: installDir,
		ConfigDir:  "config-server",
		Host:       "10.0.0.1",
		KeyFile:    "key.pem",
	})
	if err == nil {
		t.Fatalf("expected error for key without certificate")
	}
}

func TestBuildServerInstallBaseAcceptsCertFile(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config-server")
	certPath := filepath.Join(installDir, "cert.pem")
	if err := os.WriteFile(certPath, []byte("cert"), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}

	base, err := buildServerInstallBase(installDir, configDir, InstallOptions{
		InstallDir:      installDir,
		ConfigDir:       "config-server",
		Host:            "10.0.0.1",
		CertificateFile: certPath,
	})
	if err != nil {
		t.Fatalf("buildServerInstallBase error: %v", err)
	}
	if base.selfSigned {
		t.Fatalf("expected selfSigned to be false")
	}
}
