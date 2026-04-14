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

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestSetCertificateGeneratesSelfSigned(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
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

	pendingDir := mustPendingConfigDir(t, configDir)
	certPath := filepath.Join(pendingDir, "cert.pem")
	keyPath := filepath.Join(pendingDir, "key.pem")

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

	configPath := filepath.Join(pendingDir, "inbounds.json")
	liveConfigDir, err := config.LiveConfigDir(configDir)
	if err != nil {
		t.Fatalf("live config dir: %v", err)
	}
	assertTLSConfigUpdated(t, configPath, filepath.ToSlash(filepath.Join(liveConfigDir, "cert.pem")), filepath.ToSlash(filepath.Join(liveConfigDir, "key.pem")))

	data, err := os.ReadFile(configPath)
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
	tlsSettings, _ := stream["tlsSettings"].(map[string]any)
	if tlsSettings == nil {
		t.Fatalf("expected tlsSettings")
	}
	if _, ok := tlsSettings["allowInsecure"]; ok {
		t.Fatalf("did not expect allowInsecure in server tlsSettings")
	}
}

func TestSetCertificateUsesProvidedPaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
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

	pendingDir := mustPendingConfigDir(t, configDir)
	configPath := filepath.Join(pendingDir, "inbounds.json")
	liveConfigDir, err := config.LiveConfigDir(configDir)
	if err != nil {
		t.Fatalf("live config dir: %v", err)
	}
	assertTLSConfigUpdated(t, configPath, filepath.ToSlash(filepath.Join(liveConfigDir, "cert.pem")), filepath.ToSlash(filepath.Join(liveConfigDir, "key.pem")))

	data, err := os.ReadFile(configPath)
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
	tlsSettings, _ := stream["tlsSettings"].(map[string]any)
	if tlsSettings == nil {
		t.Fatalf("expected tlsSettings")
	}
	if _, ok := tlsSettings["allowInsecure"]; ok {
		t.Fatalf("did not expect allowInsecure in server tlsSettings")
	}
}

func TestSetCertificateRequiresForceWhenTLSConfigured(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
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
		tlsSettings := map[string]any{
			"certificates": []any{
				map[string]any{
					"certificateFile": "cert.pem",
					"keyFile":         "key.pem",
				},
			},
		}
		streamSettings["tlsSettings"] = tlsSettings
	}

	root := map[string]any{
		"inbounds": []any{
			map[string]any{
				"protocol":       "trojan",
				"port":           DefaultTrojanPort,
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

func assertTLSConfigUpdated(t *testing.T, path, expectedCert, expectedKey string) {
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
