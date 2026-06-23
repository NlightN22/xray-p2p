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

func TestManagedUserLabelIsDeterministic(t *testing.T) {
	first, err := ManagedUserLabel("corp", "user-123")
	if err != nil {
		t.Fatal(err)
	}
	second, err := ManagedUserLabel("corp", "user-123")
	if err != nil {
		t.Fatal(err)
	}
	otherProvider, err := ManagedUserLabel("other", "user-123")
	if err != nil {
		t.Fatal(err)
	}
	otherSubject, err := ManagedUserLabel("corp", "user-456")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("label is not stable: %q != %q", first, second)
	}
	if !IsManagedUserLabel(first) || !strings.HasSuffix(first, "@xp2p.local") {
		t.Fatalf("unexpected managed label: %q", first)
	}
	if first == otherProvider || first == otherSubject {
		t.Fatalf("label did not include provider and subject: %q", first)
	}
}

func TestManagedUserLabelRequiresStableInputs(t *testing.T) {
	if _, err := ManagedUserLabel("", "user"); err == nil {
		t.Fatal("expected provider instance id error")
	}
	if _, err := ManagedUserLabel("corp", ""); err == nil {
		t.Fatal("expected external subject error")
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
