package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
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
	if base.installOpts.Profile != "trojan-tls" {
		t.Fatalf("installOpts profile = %s", base.installOpts.Profile)
	}
	if !base.installOpts.TunEnabled {
		t.Fatalf("expected tun enabled by default")
	}
	if base.installOpts.TunName != "xp2ps" {
		t.Fatalf("expected tun name xp2ps, got %s", base.installOpts.TunName)
	}
	if base.installOpts.TunMTU != 1500 {
		t.Fatalf("expected tun MTU 1500, got %d", base.installOpts.TunMTU)
	}
	if base.installOpts.TunAddr != "198.18.0.5/30" {
		t.Fatalf("expected tun addr 198.18.0.5/30, got %s", base.installOpts.TunAddr)
	}
}

func TestBuildServerInstallBaseAppliesExplicitProfile(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config-server")

	base, err := buildServerInstallBase(installDir, configDir, InstallOptions{
		InstallDir: installDir,
		ConfigDir:  "config-server",
		Host:       "10.0.0.1",
		Profile:    "vless-tls-vision",
	})
	if err != nil {
		t.Fatalf("buildServerInstallBase error: %v", err)
	}
	if base.installOpts.Profile != "vless-tls-vision" {
		t.Fatalf("installOpts profile = %s", base.installOpts.Profile)
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
	certPath, keyPath := writeTestCertificateFiles(t, installDir, "install.local")

	base, err := buildServerInstallBase(installDir, configDir, InstallOptions{
		InstallDir:      installDir,
		ConfigDir:       "config-server",
		Host:            "10.0.0.1",
		CertificateFile: certPath,
		KeyFile:         keyPath,
	})
	if err != nil {
		t.Fatalf("buildServerInstallBase error: %v", err)
	}
	if base.selfSigned {
		t.Fatalf("expected selfSigned to be false")
	}
}

func writeTestCertificateFiles(t *testing.T, dir, host string) (string, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: host,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{host},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPath := filepath.Join(dir, "source-cert.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}

	keyPath := filepath.Join(dir, "source-key.pem")
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	return certPath, keyPath
}
