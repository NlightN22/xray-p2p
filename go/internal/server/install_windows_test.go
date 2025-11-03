//go:build windows

package server

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizeInstallOptionsRequiresHost(t *testing.T) {
	_, err := normalizeInstallOptions(InstallOptions{
		InstallDir: filepath.Join(t.TempDir(), "srv"),
		ConfigDir:  "cfg",
		Port:       "62022",
	})
	if err == nil {
		t.Fatalf("expected error when host is missing")
	}
}

func TestInstallGeneratesSelfSignedCertificate(t *testing.T) {
	t.Helper()

	installDir := filepath.Join(t.TempDir(), "srv-domain")
	opts := InstallOptions{
		InstallDir: installDir,
		ConfigDir:  "config-server",
		Port:       "62022",
		Host:       "example.test",
	}

	if err := Install(context.Background(), opts); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	certPath := filepath.Join(installDir, "config-server", "cert.pem")
	cert := loadCertificate(t, certPath)
	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != "example.test" {
		t.Fatalf("expected certificate DNSNames to contain example.test, got %v", cert.DNSNames)
	}
	if len(cert.IPAddresses) != 0 {
		t.Fatalf("expected no IP addresses, got %v", cert.IPAddresses)
	}

	if cert.NotAfter.Before(time.Now().AddDate(9, 0, 0)) {
		t.Fatalf("expected certificate validity of approximately 10 years, got %v", cert.NotAfter.Sub(cert.NotBefore))
	}
}

func TestInstallGeneratesSelfSignedCertificateForIP(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "srv-ip")
	host := "203.0.113.5"
	opts := InstallOptions{
		InstallDir: installDir,
		ConfigDir:  "config-server",
		Port:       "62022",
		Host:       host,
	}

	if err := Install(context.Background(), opts); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	certPath := filepath.Join(installDir, "config-server", "cert.pem")
	cert := loadCertificate(t, certPath)
	if len(cert.DNSNames) != 0 {
		t.Fatalf("expected no DNS names for IP host, got %v", cert.DNSNames)
	}

	found := false
	for _, ip := range cert.IPAddresses {
		if ip.Equal(net.ParseIP(host)) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected certificate to contain IP %s, got %v", host, cert.IPAddresses)
	}
}

func loadCertificate(t *testing.T, path string) *x509.Certificate {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read certificate: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatalf("decode certificate PEM failed")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}
