//go:build windows

package server

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestSetCertificateGeneratesSelfSigned(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

	if err := os.MkdirAll(filepath.Join(dir, layout.ServerConfigDir), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}

	opts := CertificateOptions{
		InstallDir: dir,
		ConfigDir:  layout.ServerConfigDir,
		Host:       "cert.test.local",
		Force:      true,
	}

	if err := SetCertificate(context.Background(), opts); err != nil {
		t.Fatalf("SetCertificate failed: %v", err)
	}

	certPath := defaultCertPath()
	keyPath := defaultKeyPath()

	cert := loadCertificateFile(t, certPath)
	if cert.Subject.CommonName != "cert.test.local" {
		t.Fatalf("expected CommonName cert.test.local, got %s", cert.Subject.CommonName)
	}
	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != "cert.test.local" {
		t.Fatalf("expected DNS SAN with cert.test.local, got %v", cert.DNSNames)
	}
	if time.Until(cert.NotAfter) < 9*365*24*time.Hour {
		t.Fatalf("expected certificate validity close to 10 years, got %s", time.Until(cert.NotAfter))
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if !pemContainsBlock(keyData, "RSA PRIVATE KEY") {
		t.Fatalf("expected RSA private key in %s", keyPath)
	}

	if _, err := os.Stat(config.ApplyRequestPath()); err != nil {
		t.Fatalf("expected apply request: %v", err)
	}
}

func TestSetCertificateUsesProvidedPaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

	if err := os.MkdirAll(filepath.Join(dir, layout.ServerConfigDir), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}

	srcCert, srcKey := createTestCertificateFiles(t, dir, "provided.example.test")

	opts := CertificateOptions{
		InstallDir:      dir,
		ConfigDir:       layout.ServerConfigDir,
		CertificateFile: srcCert,
		KeyFile:         srcKey,
		Force:           true,
	}

	if err := SetCertificate(context.Background(), opts); err != nil {
		t.Fatalf("SetCertificate failed: %v", err)
	}

	if _, err := os.Stat(defaultCertPath()); err != nil {
		t.Fatalf("expected default cert to be created: %v", err)
	}
	if _, err := os.Stat(defaultKeyPath()); err != nil {
		t.Fatalf("expected default key to be created: %v", err)
	}
}

func TestSetCertificateRequiresForceWhenTLSConfigured(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

	if err := os.MkdirAll(filepath.Join(dir, layout.ServerConfigDir), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}

	if err := os.MkdirAll(defaultTLSDir(), 0o755); err != nil {
		t.Fatalf("mkdir tls dir: %v", err)
	}
	writeCertificateFile(t, defaultCertPath(), defaultKeyPath(), "force.test", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	opts := CertificateOptions{
		InstallDir: dir,
		ConfigDir:  layout.ServerConfigDir,
		Host:       "force.test",
	}

	err := SetCertificate(context.Background(), opts)
	if err == nil || err != ErrCertificateConfigured {
		t.Fatalf("expected ErrCertificateConfigured, got %v", err)
	}
}

func loadCertificateFile(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read certificate: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatalf("decode pem: nil")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}

func pemContainsBlock(data []byte, blockType string) bool {
	for {
		block, rest := pem.Decode(data)
		if block == nil {
			return false
		}
		if block.Type == blockType {
			return true
		}
		data = rest
	}
}
