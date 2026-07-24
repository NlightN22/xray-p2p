package root

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func serverCertStateContractCase() contractCase {
	args := []string{"server", "cert", "state"}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup:    setupServerCertStateCase,
		assertResult: func(t *testing.T, result map[string]any) {
			if result["status"] != "ok" || result["self_signed"] != true ||
				result["remaining_days"].(float64) <= 0 ||
				result["subject"] != "CN=matrix Ω certificate\x01" {
				t.Fatalf("certificate state changed: %#v", result)
			}
			dns, ok := result["dns_names"].([]any)
			if !ok || len(dns) != 2 || dns[0] != "zulu.example" || dns[1] != "alpha example" {
				t.Fatalf("certificate DNS names changed: %#v", result["dns_names"])
			}
			ips, ok := result["ip_addresses"].([]any)
			if !ok || len(ips) != 1 || ips[0] != "192.0.2.10" {
				t.Fatalf("certificate IP addresses changed: %#v", result["ip_addresses"])
			}
			for _, key := range []string{"not_before", "not_after"} {
				value, ok := result[key].(string)
				if !ok {
					t.Fatalf("%s is not a string: %#v", key, result[key])
				}
				parsed, err := time.Parse(time.RFC3339, value)
				if err != nil || parsed.Location() != time.UTC {
					t.Fatalf("%s is not UTC RFC3339: %q (%v)", key, value, err)
				}
			}
			if strings.Contains(fmt.Sprintf("%v", result), "PRIVATE KEY") {
				t.Fatalf("certificate state leaked private key material: %#v", result)
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			for _, key := range []string{"dns_names", "ip_addresses", "issues"} {
				items, ok := result[key].([]any)
				if !ok || items == nil || len(items) != 0 {
					t.Fatalf("empty %s must be []: %#v", key, result[key])
				}
			}
			if result["status"] != "ok" || result["self_signed"] != true {
				t.Fatalf("minimal valid certificate state changed: %#v", result)
			}
		},
		emptyResult:      "a minimal valid certificate reports non-nil empty SAN and issue arrays",
		credentialPolicy: "state exposes paths and public certificate metadata but never private key contents",
		edgeCases:        []string{"number", "boolean", "UTC timestamps", "empty arrays", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{"Certificate:", "Key:", "matrix Ω certificate", "DNS: zulu.example, alpha example", "192.0.2.10", "Self-signed: yes", "Status:      OK"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human baseline is missing %q: output=%q diagnostics=%q", expected, output, diagnostics)
				}
			}
		},
	}
}

func setupServerCertStateCase(t *testing.T, mode string) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	certPath := filepath.Join(root, "tls", "server.crt")
	keyPath := filepath.Join(root, "tls", "server.key")
	fixture := fmt.Sprintf("[server]\ncertificate = %q\nkey = %q\n",
		filepath.ToSlash(certPath), filepath.ToSlash(keyPath))
	writeContractFixture(t, filepath.Join(root, layout.ServerConfigFileName), fixture)
	if err := os.MkdirAll(filepath.Dir(certPath), 0o755); err != nil {
		t.Fatalf("create TLS fixture directory: %v", err)
	}
	if mode == "error" {
		writeContractFixture(t, certPath, "invalid certificate")
		writeContractFixture(t, keyPath, "key exists")
		return
	}
	commonName := "minimal"
	var dnsNames []string
	var ipAddresses []net.IP
	if mode == "success" {
		commonName = "matrix Ω certificate\x01"
		dnsNames = []string{"zulu.example", "alpha example"}
		ipAddresses = []net.IP{net.ParseIP("192.0.2.10")}
	}
	writeContractCertificate(t, certPath, keyPath, commonName, dnsNames, ipAddresses)
}

func writeContractCertificate(t *testing.T, certPath, keyPath, commonName string, dnsNames []string, ipAddresses []net.IP) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate certificate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(7),
		Subject:      pkix.Name{CommonName: commonName},
		Issuer:       pkix.Name{CommonName: commonName},
		NotBefore:    time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
		NotAfter:     time.Date(2035, 1, 2, 3, 4, 5, 0, time.UTC),
		DNSNames:     dnsNames,
		IPAddresses:  ipAddresses,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal certificate key: %v", err)
	}
	writeContractFixture(t, certPath, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
	writeContractFixture(t, keyPath, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})))
}
