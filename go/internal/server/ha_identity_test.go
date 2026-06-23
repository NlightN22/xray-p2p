package server

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
)

func TestApplyHAIdentityStateStoresGenerationAndProvisionedOverlay(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	generation := identitysync.Generation{
		ID:                 "gen-1",
		ProviderInstanceID: "idp",
		Subjects: map[string]identitysync.Subject{
			"alice": {ExternalSubject: "alice", UserLabel: "idp-alice@xp2p.local", Active: true},
			"bob":   {ExternalSubject: "bob", UserLabel: "idp-bob@xp2p.local", Active: true, Provisioned: true},
		},
		Groups: map[string]identitysync.Group{"users": {ID: "users", DirectMembers: []string{"alice", "bob"}}},
	}
	identityPayload, err := json.Marshal(generation)
	if err != nil {
		t.Fatal(err)
	}
	provisionedPayload, err := json.Marshal([]string{"idp-alice@xp2p.local"})
	if err != nil {
		t.Fatal(err)
	}
	if err := applyHAIdentityState(identityPayload, provisionedPayload); err != nil {
		t.Fatal(err)
	}
	state, err := identitysync.DefaultStore().Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Current == nil || state.Current.ID != "gen-1" {
		t.Fatalf("state = %+v", state)
	}
	if !state.Current.Subjects["alice"].Provisioned || state.Current.Subjects["bob"].Provisioned {
		t.Fatalf("subjects = %+v", state.Current.Subjects)
	}
}
