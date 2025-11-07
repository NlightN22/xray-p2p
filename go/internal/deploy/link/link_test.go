package link

import (
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
)

func TestBuildParseRoundTrip(t *testing.T) {
	manifest := spec.Manifest{
		Host:           "10.0.10.10",
		Version:        2,
		InstallDir:     "/opt/xp2p",
		TrojanPort:     "58443",
		TrojanUser:     "user@example.invalid",
		TrojanPassword: "secret",
		ExpiresAt:      1_900_000_000,
	}

	url, enc, err := Build("10.0.10.10", "62025", manifest, time.Minute)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty link URL")
	}
	if enc.Key == "" || enc.Nonce == "" || enc.CiphertextB64 == "" {
		t.Fatalf("encrypted link missing fields: %#v", enc)
	}

	parsed, err := Parse(url)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed.Manifest != manifest {
		t.Fatalf("manifest mismatch: %#v != %#v", parsed.Manifest, manifest)
	}
	if parsed.Port != "62025" {
		t.Fatalf("unexpected port %q", parsed.Port)
	}
	if parsed.CiphertextB64 != enc.CiphertextB64 {
		t.Fatalf("ciphertext mismatch")
	}
}

func TestParseRejectsInvalidVersion(t *testing.T) {
	url, _, err := Build("host", "62025", spec.Manifest{
		Host:    "host",
		Version: 2,
	}, time.Minute)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	broken := strings.Replace(url, "v=2", "v=3", 1)
	if _, err := Parse(broken); err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestRedactedURLOmitsKey(t *testing.T) {
	_, enc, err := Build("host", "62025", spec.Manifest{
		Host:    "host",
		Version: 2,
	}, time.Minute)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	redacted := RedactedURL(enc.Host, enc.Port, enc.Nonce, enc.ExpiresAt, enc.CiphertextB64)
	if strings.Contains(redacted, enc.Key) {
		t.Fatalf("redacted link leaks key: %s", redacted)
	}
	if !strings.Contains(redacted, enc.CiphertextB64) {
		t.Fatalf("redacted link missing ciphertext")
	}
}
