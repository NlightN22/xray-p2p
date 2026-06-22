//go:build linux || windows

package server

import (
	"context"
	"os"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

func TestForceRotateLegacyCredentialsIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	doc := map[string]any{}
	setServerUsers(doc, []trojanClient{{Email: "legacy", Password: "legacy-password"}, {Email: "uuid", Password: "550e8400-e29b-41d4-a716-446655440000"}})
	if err := writeServerStateDoc(config.ConfigPath(layout.ServerConfigFileName), doc); err != nil {
		t.Fatalf("write desired state: %v", err)
	}
	if err := ForceRotateLegacyCredentials(context.Background()); err != nil {
		t.Fatalf("force rotate: %v", err)
	}
	first, err := loadServerDesiredConfigFromPath(pendingConfigPath())
	if err != nil {
		t.Fatalf("load rotated state: %v", err)
	}
	if !tunnel.IsUUIDCredential(first.Users[0].Password) || first.Users[0].PreviousCredentialForRotation != "legacy-password" || first.Users[0].CredentialGeneration != 2 {
		t.Fatalf("legacy user not rotated: %#v", first.Users[0])
	}
	if first.Users[1].Password != "550e8400-e29b-41d4-a716-446655440000" || first.Users[1].CredentialGeneration != 1 {
		t.Fatalf("UUID user changed: %#v", first.Users[1])
	}
	firstCredential := first.Users[0].Password
	if err := ForceRotateLegacyCredentials(context.Background()); err != nil {
		t.Fatalf("second force rotate: %v", err)
	}
	second, err := loadServerDesiredConfigFromPath(pendingConfigPath())
	if err != nil {
		t.Fatalf("load idempotent state: %v", err)
	}
	if second.Users[0].Password != firstCredential || second.Users[0].CredentialGeneration != 2 {
		t.Fatalf("forced rotation repeated: %#v", second.Users[0])
	}
	if _, err := os.Stat(config.ApplyRequestPath()); !os.IsNotExist(err) {
		t.Fatalf("staged rotation must not publish an apply request: %v", err)
	}
}

func TestStageLegacyCredentialRotationWritesApplyRequest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	doc := map[string]any{}
	setServerUsers(doc, []trojanClient{{Email: "legacy", Password: "legacy-password"}})
	if err := writeServerStateDoc(config.ConfigPath(layout.ServerConfigFileName), doc); err != nil {
		t.Fatalf("write desired state: %v", err)
	}
	if err := StageLegacyCredentialRotation(context.Background()); err != nil {
		t.Fatalf("stage rotate: %v", err)
	}
	if _, err := os.Stat(config.ApplyRequestPath()); err != nil {
		t.Fatalf("expected apply request: %v", err)
	}
}
