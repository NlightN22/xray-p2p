//go:build windows || linux

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
	"strings"
	"testing"
	"time"
)

func TestCertificateStateStatuses(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name        string
		notBefore   time.Time
		notAfter    time.Time
		corruptCert bool
		createCert  bool
		wantStatus  CertificateStatus
	}{
		{
			name:       "valid certificate",
			notBefore:  now.Add(-time.Hour),
			notAfter:   now.Add(48 * time.Hour),
			createCert: true,
			wantStatus: CertificateStatusOK,
		},
		{
			name:       "expired certificate",
			notBefore:  now.Add(-48 * time.Hour),
			notAfter:   now.Add(-time.Hour),
			createCert: true,
			wantStatus: CertificateStatusExpired,
		},
		{
			name:       "missing certificate",
			createCert: false,
			wantStatus: CertificateStatusMissing,
		},
		{
			name:        "corrupt certificate",
			createCert:  true,
			corruptCert: true,
			wantStatus:  CertificateStatusParseError,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			configDir := filepath.Join(dir, "config-server")
			writeTLSConfig(t, configDir, "cert.pem", "key.pem")

			certPath := filepath.Join(configDir, "cert.pem")
			keyPath := filepath.Join(configDir, "key.pem")

			if tc.createCert {
				if tc.corruptCert {
					if err := os.WriteFile(certPath, []byte("broken"), 0o644); err != nil {
						t.Fatalf("write corrupt cert: %v", err)
					}
				} else {
					writeCertificateFile(t, certPath, keyPath, "status.test", tc.notBefore, tc.notAfter)
				}
			} else {
				if err := os.WriteFile(keyPath, []byte("dummy key"), 0o600); err != nil {
					t.Fatalf("write key: %v", err)
				}
			}

			state, err := CertificateStateFromConfig(CertificateStateOptions{
				InstallDir: dir,
				ConfigDir:  "config-server",
			})
			if err != nil {
				t.Fatalf("CertificateStateFromConfig returned error: %v", err)
			}

			if state.Status != tc.wantStatus {
				t.Fatalf("unexpected status: got %s want %s", state.Status, tc.wantStatus)
			}

			switch tc.wantStatus {
			case CertificateStatusOK:
				if len(state.Issues) != 0 {
					t.Fatalf("expected no issues, got %v", state.Issues)
				}
				if !strings.Contains(state.Subject, "status.test") {
					t.Fatalf("unexpected subject %q", state.Subject)
				}
				if state.RemainingDays < 1 {
					t.Fatalf("expected positive remaining days, got %d", state.RemainingDays)
				}
			case CertificateStatusExpired:
				if state.RemainingDays >= 0 {
					t.Fatalf("expected negative remaining days for expired certificate")
				}
			case CertificateStatusMissing, CertificateStatusParseError:
				if len(state.Issues) == 0 {
					t.Fatalf("expected issues for status %s", state.Status)
				}
			}
		})
	}
}

func writeTLSConfig(t *testing.T, configDir, certName, keyName string) {
	t.Helper()
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", configDir, err)
	}

	streamSettings := map[string]any{
		"security": "tls",
		"tlsSettings": map[string]any{
			"certificates": []any{
				map[string]any{
					"certificateFile": certName,
					"keyFile":         keyName,
				},
			},
		},
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

func writeCertificateFile(t *testing.T, certPath, keyPath, host string, notBefore, notAfter time.Time) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: host},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{host},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
}
