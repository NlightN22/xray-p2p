package identity

import (
	"strings"
	"testing"
)

func TestNewTunnelCredentialReturnsUUID(t *testing.T) {
	credential, err := NewTunnelCredential()
	if err != nil {
		t.Fatal(err)
	}
	if len(credential) != 36 || credential[14] != '4' || credential[8] != '-' || credential[13] != '-' || credential[18] != '-' || credential[23] != '-' {
		t.Fatalf("not UUID v4: %q", credential)
	}
}

func TestNewRequestIDReturnsUUID(t *testing.T) {
	id, err := NewRequestID()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 36 || id[14] != '4' {
		t.Fatalf("not UUID v4: %q", id)
	}
}

func TestNewUserIDUsesManagedLocalLabel(t *testing.T) {
	id, err := NewUserID()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "client-") || !strings.HasSuffix(id, "@xp2p.local") {
		t.Fatalf("unexpected user id: %q", id)
	}
}

func TestNewSecretReturnsURLSafeValue(t *testing.T) {
	secret, err := NewSecret(18)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(secret) == "" || strings.ContainsAny(secret, "+/=") {
		t.Fatalf("unexpected secret: %q", secret)
	}
}
