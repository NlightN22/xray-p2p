package link

import (
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
)

func TestBuildAndDecryptRoundTrip(t *testing.T) {
	manifest := spec.Manifest{
		Host:           "10.0.10.10",
		Version:        2,
		InstallDir:     "/opt/xp2p",
		TrojanPort:     "58443",
		TrojanUser:     "user@example.invalid",
		TrojanPassword: "secret",
		ExpiresAt:      1_900_000_000,
	}

	linkURL, enc, err := Build("10.0.10.10", "62025", manifest, time.Minute)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if linkURL == "" || enc.Link != linkURL {
		t.Fatalf("expected canonical link to be stored: %q vs %#v", linkURL, enc)
	}
	if len(enc.Ciphertext) == 0 {
		t.Fatal("ciphertext missing from encrypted link")
	}

	parsed, err := Parse("\n" + linkURL + "\n")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed.Link != linkURL {
		t.Fatalf("expected canonical link, got %q", parsed.Link)
	}

	got, err := Decrypt(parsed.Link, enc.Ciphertext)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if got.Host != manifest.Host || got.TrojanUser != manifest.TrojanUser || got.TrojanPassword != manifest.TrojanPassword {
		t.Fatalf("decrypted manifest mismatch: %#v", got)
	}
}

func TestCanonicalLinkRequiresCredentials(t *testing.T) {
	_, err := CanonicalLink(spec.Manifest{
		Host:       "10.0.0.1",
		Version:    2,
		TrojanPort: "62022",
	})
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestParseRejectsMissingParts(t *testing.T) {
	if _, err := Parse(""); err != nil {
		t.Fatalf("empty link should return zero value: %v", err)
	}
	if _, err := Parse("trojan://@host:62022"); err == nil {
		t.Fatal("expected error for missing password")
	}
	if _, err := Parse("https://host:62022"); err == nil {
		t.Fatal("expected error for wrong scheme")
	}
}
