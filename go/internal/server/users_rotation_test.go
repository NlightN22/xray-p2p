//go:build linux || windows

package server

import (
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

func TestRotateUserCredentialCreatesMigrationWindow(t *testing.T) {
	user := trojanClient{Email: "migration@example.test", Password: "old-credential", CredentialGeneration: 1}
	before := time.Now().UTC()

	if err := rotateUserCredential(&user, time.Hour); err != nil {
		t.Fatalf("rotate credential: %v", err)
	}

	if !tunnel.IsUUIDCredential(user.Password) {
		t.Fatalf("active credential is not a UUID: %q", user.Password)
	}
	if user.PreviousCredentialForRotation != "old-credential" {
		t.Fatalf("previous credential was not retained: %#v", user)
	}
	if user.CredentialGeneration != 2 {
		t.Fatalf("credential generation was not incremented: %#v", user)
	}
	if !user.RotationExpiresAt.After(before.Add(59 * time.Minute)) {
		t.Fatalf("rotation window is too short: %s", user.RotationExpiresAt)
	}
}

func TestSetRotationWindowWithoutPreviousCredential(t *testing.T) {
	user := trojanClient{PreviousCredentialForRotation: "stale", CredentialGeneration: 0}

	setRotationWindow(&user, "", time.Minute)

	if user.PreviousCredentialForRotation != "" || user.CredentialGeneration != 1 {
		t.Fatalf("unexpected empty-previous rotation state: %#v", user)
	}
}
