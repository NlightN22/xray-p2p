//go:build windows

package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSetCertificateGeneratesSelfSigned(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, false)

	opts := CertificateOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		Host:       "cert.test.local",
		Force:      true,
	}

	if err := SetCertificate(context.Background(), opts); err != nil {
		t.Fatalf("SetCertificate failed: %v", err)
	}

	certPath := filepath.Join(configDir, "cert.pem")
	keyPath := filepath.Join(configDir, "key.pem")

	cert := loadCertificateFile(t, certPath)
	if cert.Subject.CommonName != "cert.test.local" {
		t.Fatalf("expected CommonName cert.test.local, got %s", cert.Subject.CommonName)
	}
	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != "cert.test.local" {
		t.Fatalf("expected DNS SAN with cert.test.local, got %v", cert.DNSNames)
	}

	if time.Until(cert.NotAfter) < 9*365*24*time.Hour {
		t.Fatalf("expected certificate validity close to 10 years, got %s", cert.NotAfter.Sub(time.Now()))
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if !pemContainsBlock(keyData, "RSA PRIVATE KEY") {
		t.Fatalf("expected RSA private key in %s", keyPath)
	}

	assertTLSConfigUpdated(t, filepath.Join(configDir, "inbounds.json"))
}

func TestSetCertificateCopiesProvidedFiles(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, false)

	srcCert, srcKey := createTestCertificateFiles(t, dir, "provided.example.test")

	opts := CertificateOptions{
		InstallDir:      dir,
		ConfigDir:       "config-server",
		CertificateFile: srcCert,
		KeyFile:         srcKey,
		Host:            "",
		Force:           true,
	}

	if err := SetCertificate(context.Background(), opts); err != nil {
		t.Fatalf("SetCertificate failed: %v", err)
	}

	destCert := filepath.Join(configDir, "cert.pem")
	destKey := filepath.Join(configDir, "key.pem")

	destCertData, err := os.ReadFile(destCert)
	if err != nil {
		t.Fatalf("read dest cert: %v", err)
	}
	srcCertData, _ := os.ReadFile(srcCert)
	if string(destCertData) != string(srcCertData) {
		t.Fatalf("expected certificate contents to match source")
	}

	destKeyData, err := os.ReadFile(destKey)
	if err != nil {
		t.Fatalf("read dest key: %v", err)
	}
	srcKeyData, _ := os.ReadFile(srcKey)
	if string(destKeyData) != string(srcKeyData) {
		t.Fatalf("expected key contents to match source")
	}

	assertTLSConfigUpdated(t, filepath.Join(configDir, "inbounds.json"))
}

func TestSetCertificateRequiresForceWhenTLSConfigured(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, true)

	opts := CertificateOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		Host:       "force.test",
	}

	err := SetCertificate(context.Background(), opts)
	if err == nil || err != ErrCertificateConfigured {
		t.Fatalf("expected ErrCertificateConfigured, got %v", err)
	}
}

func prepareTrojanConfig(t *testing.T, configDir string, withTLS bool) {
	t.Helper()
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", configDir, err)
	}

	streamSettings := map[string]any{
		"security": "none",
	}
	if withTLS {
		streamSettings["security"] = "tls"
		streamSettings["tlsSettings"] = map[string]any{
			"certificates": []any{
				map[string]any{
					"certificateFile": "cert.pem",
					"keyFile":         "key.pem",
				},
			},
		}
	}

	root := map[string]any{
		"inbounds": []any{
			map[string]any{
				"protocol":       "trojan",
				"port":           62022,
				"streamSettings": streamSettings,
				"settings": map[string]any{
					"clients": []any{},
				},
			},
		},
	}

	if err := writeInbounds(filepath.Join(configDir, "inbounds.json"), root); err != nil {
		t.Fatalf("write inbounds: %v", err)
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

func assertTLSConfigUpdated(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read inbounds: %v", err)
	}
	root, err := parseInbounds(data)
	if err != nil {
		t.Fatalf("parse inbounds: %v", err)
	}
	trojan, err := selectTrojanInbound(root)
	if err != nil {
		t.Fatalf("select trojan: %v", err)
	}
	stream, err := extractStreamSettings(trojan)
	if err != nil {
		t.Fatalf("extract stream: %v", err)
	}
	if !hasTLSConfigured(stream) {
		t.Fatalf("expected TLS security enabled")
	}
	tlsSettings, _ := stream["tlsSettings"].(map[string]any)
	if tlsSettings == nil {
		t.Fatalf("expected tlsSettings")
	}
	certs, _ := tlsSettings["certificates"].([]any)
	if len(certs) == 0 {
		t.Fatalf("expected certificates entry")
	}
	entry, _ := certs[0].(map[string]any)
	configDir := filepath.Dir(path)
	expectedCert := filepath.ToSlash(filepath.Join(configDir, "cert.pem"))
	expectedKey := filepath.ToSlash(filepath.Join(configDir, "key.pem"))
	if entry["certificateFile"] != expectedCert {
		t.Fatalf("unexpected certificateFile: %v", entry["certificateFile"])
	}
	if entry["keyFile"] != expectedKey {
		t.Fatalf("unexpected keyFile: %v", entry["keyFile"])
	}
}

func createTestCertificateFiles(t *testing.T, dir, host string) (string, string) {
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
