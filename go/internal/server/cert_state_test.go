//go:build windows || linux

package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
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
			t.Setenv("XP2P_CONFIG_ROOT", dir)
			if err := os.MkdirAll(filepath.Join(dir, layout.ServerConfigDir), 0o755); err != nil {
				t.Fatalf("mkdir config dir: %v", err)
			}
			if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
				t.Fatalf("write server config: %v", err)
			}

			certPath := defaultCertPath()
			keyPath := defaultKeyPath()

			if tc.createCert {
				if err := os.MkdirAll(defaultTLSDir(), 0o755); err != nil {
					t.Fatalf("mkdir tls dir: %v", err)
				}
				if tc.corruptCert {
					if err := os.WriteFile(certPath, []byte("broken"), 0o644); err != nil {
						t.Fatalf("write corrupt cert: %v", err)
					}
					if err := os.WriteFile(keyPath, []byte("dummy key"), 0o600); err != nil {
						t.Fatalf("write key: %v", err)
					}
				} else {
					writeCertificateFile(t, certPath, keyPath, "status.test", tc.notBefore, tc.notAfter)
				}
			}

			state, err := CertificateStateFromConfig(CertificateStateOptions{
				InstallDir: dir,
				ConfigDir:  layout.ServerConfigDir,
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
				if !state.SelfSigned {
					t.Fatalf("expected self-signed certificate")
				}
			case CertificateStatusExpired:
				if state.RemainingDays >= 0 {
					t.Fatalf("expected negative remaining days for expired certificate")
				}
				if !state.SelfSigned {
					t.Fatalf("expected self-signed certificate")
				}
			case CertificateStatusMissing, CertificateStatusParseError:
				if len(state.Issues) == 0 {
					t.Fatalf("expected issues for status %s", state.Status)
				}
				if state.SelfSigned {
					t.Fatalf("did not expect self-signed certificate")
				}
			}
		})
	}
}
