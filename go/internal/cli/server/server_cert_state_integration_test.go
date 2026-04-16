package servercmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestServerCertStateCommand(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		notBefore  time.Time
		notAfter   time.Time
		wantStatus string
		wantSigned string
		wantCode   int
	}{
		{
			name:       "valid certificate",
			notBefore:  now.Add(-time.Hour),
			notAfter:   now.Add(24 * time.Hour),
			wantStatus: "Status:      OK",
			wantSigned: "Self-signed: yes",
			wantCode:   0,
		},
		{
			name:       "expired certificate",
			notBefore:  now.Add(-48 * time.Hour),
			notAfter:   now.Add(-time.Hour),
			wantStatus: "Status:      EXPIRED",
			wantSigned: "Self-signed: yes",
			wantCode:   1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", dir)
			if err := os.WriteFile(filepath.Join(dir, layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
				t.Fatalf("write server config: %v", err)
			}
			tlsDir := filepath.Join(dir, "tls", "server")
			if err := os.MkdirAll(tlsDir, 0o755); err != nil {
				t.Fatalf("mkdir tls dir: %v", err)
			}
			certPath := filepath.Join(tlsDir, "cert.pem")
			keyPath := filepath.Join(tlsDir, "key.pem")
			writeTestCertificate(t, certPath, keyPath, tt.notBefore, tt.notAfter, "cli.state.test")

			cfg := config.Config{
				Server: config.ServerConfig{
					InstallDir: dir,
					ConfigDir:  layout.ServerConfigDir,
				},
			}

			output, code := executeCertState(t, cfg, "--path", dir, "--config-dir", layout.ServerConfigDir)

			if code != tt.wantCode {
				t.Fatalf("exit code: got %d want %d\noutput:\n%s", code, tt.wantCode, output)
			}
			if !strings.Contains(output, tt.wantStatus) {
				t.Fatalf("output did not include status %q\noutput:\n%s", tt.wantStatus, output)
			}
			if !strings.Contains(output, tt.wantSigned) {
				t.Fatalf("output did not include self-signed line %q\noutput:\n%s", tt.wantSigned, output)
			}
			if !strings.Contains(output, certPath) {
				t.Fatalf("output missing cert path %s", certPath)
			}
		})
	}
}

func executeCertState(t *testing.T, cfg config.Config, args ...string) (string, int) {
	t.Helper()
	var code int
	output := captureStdout(t, func() {
		root := newServerTestRoot(NewCommand(func() config.Config { return cfg }))
		root.SetArgs(append([]string{"server", "cert", "state"}, args...))
		err := root.Execute()
		if err != nil {
			var exit exitError
			if !errors.As(err, &exit) {
				t.Fatalf("unexpected error: %v", err)
			}
			code = exit.ExitCode()
			return
		}
		code = 0
	})
	return output, code
}

func writeTestCertificate(t *testing.T, certPath, keyPath string, notBefore, notAfter time.Time, host string) {
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
