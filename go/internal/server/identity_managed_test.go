//go:build windows || linux

package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestRemoveManagedUserClearsProvisionedState(t *testing.T) {
	dir := setupManagedIdentityTest(t)
	writeManagedIdentityState(t, "idp-alice@xp2p.local", true)
	writeServerStateFile(t, dir, map[string]serverReverseChannel{
		"idp-alice-example.rev": {UserID: "idp-alice@xp2p.local", Host: "example.com", Tag: "idp-alice-example.rev", Domain: "idp-alice-example.rev"},
	}, nil)
	doc := readServerStateDoc(t, pendingConfigPath())
	setServerUsers(doc, []trojanClient{{Email: "idp-alice@xp2p.local", Password: "secret", ManagedByIdentity: true}})
	if err := writeServerStateDoc(pendingConfigPath(), doc); err != nil {
		t.Fatalf("write state: %v", err)
	}

	if err := RemoveUser(context.Background(), RemoveUserOptions{UserID: "idp-alice@xp2p.local"}); err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}
	state, err := identitysync.DefaultStore().Load()
	if err != nil {
		t.Fatalf("load identity: %v", err)
	}
	if state.Current.Subjects["alice"].Provisioned {
		t.Fatalf("provisioned was not cleared: %+v", state.Current.Subjects["alice"])
	}
}

func TestAuthoritativeIdentityRemovalCascadesOwnedResources(t *testing.T) {
	dir := setupManagedIdentityTest(t)
	writeManagedIdentityState(t, "idp-alice@xp2p.local", true)
	writeServerStateFile(t, dir, map[string]serverReverseChannel{
		"idp-alice-example.rev": {UserID: "idp-alice@xp2p.local", Host: "example.com", Tag: "idp-alice-example.rev", Domain: "idp-alice-example.rev"},
	}, []map[string]any{
		{"domain": "svc.example", "outbound_tag": "idp-alice-example.rev"},
	})
	doc := readServerStateDoc(t, pendingConfigPath())
	setServerUsers(doc, []trojanClient{{Email: "idp-alice@xp2p.local", Password: "secret", ManagedByIdentity: true}})
	if err := writeServerStateDoc(pendingConfigPath(), doc); err != nil {
		t.Fatalf("write state: %v", err)
	}

	if err := RemoveAuthoritativeIdentity(context.Background(), "idp-alice@xp2p.local"); err != nil {
		t.Fatalf("RemoveAuthoritativeIdentity: %v", err)
	}
	desired, err := loadServerDesiredConfigFromPath(pendingConfigPath())
	if err != nil {
		t.Fatalf("load desired: %v", err)
	}
	if len(desired.Users) != 0 || len(desired.Reverse) != 0 || len(desired.Redirects) != 0 {
		t.Fatalf("owned resources remain: %+v", desired)
	}
	state, err := identitysync.DefaultStore().Load()
	if err != nil {
		t.Fatalf("load identity: %v", err)
	}
	if _, ok := state.Current.Subjects["alice"]; ok {
		t.Fatalf("identity subject remains: %+v", state.Current.Subjects)
	}
}

func setupManagedIdentityTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	return dir
}

func writeManagedIdentityState(t *testing.T, label string, provisioned bool) {
	t.Helper()
	if err := identitysync.DefaultStore().Save(identitysync.State{
		Current: &identitysync.Generation{
			ID:                 "gen-1",
			ProviderInstanceID: "provider-1",
			Subjects: map[string]identitysync.Subject{
				"alice": {ExternalSubject: "alice", UserLabel: label, Active: true, Provisioned: provisioned},
			},
			Groups: map[string]identitysync.Group{},
		},
	}); err != nil {
		t.Fatalf("write identity state: %v", err)
	}
}
