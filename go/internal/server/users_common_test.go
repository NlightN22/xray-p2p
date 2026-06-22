package server

import (
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

func TestDecodeServerUsersReadsNeutralRecords(t *testing.T) {
	users, err := decodeServerTrojanUsers(map[string]any{
		serverUsersKey: []any{map[string]any{
			"user_label": "alice",
			"credential": "550e8400-e29b-41d4-a716-446655440000",
			"disabled":   true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Email != "alice" || users[0].Password != "550e8400-e29b-41d4-a716-446655440000" || !users[0].Disabled {
		t.Fatalf("unexpected users: %#v", users)
	}
}

func TestDecodeServerUsersNormalizesLegacyTrojanRecords(t *testing.T) {
	users, err := decodeServerTrojanUsers(map[string]any{
		serverTrojanUsersKey: []any{map[string]any{"email": "alice", "password": "secret"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Email != "alice" || users[0].Password != "secret" {
		t.Fatalf("unexpected users: %#v", users)
	}
}

func TestSetServerUsersWritesNeutralRecords(t *testing.T) {
	doc := map[string]any{}
	setServerUsers(doc, []trojanClient{{Email: "alice", Password: "secret"}})
	if doc[serverTrojanUsersKey] != nil {
		t.Fatalf("legacy users remained: %#v", doc)
	}
	users, ok := doc[serverUsersKey].([]tunnel.User)
	if !ok || len(users) != 1 || users[0].UserLabel != "alice" || users[0].ActiveCredential != "secret" || users[0].CredentialGeneration != 1 {
		t.Fatalf("unexpected neutral users: %#v", doc[serverUsersKey])
	}
}

func TestDecodeServerUsersAllowsIdentityManagedMetadata(t *testing.T) {
	users, err := decodeServerTrojanUsers(map[string]any{
		serverUsersKey: []any{map[string]any{
			"user_label":        "idp-alice@xp2p.local",
			"active_credential": "secret",
			"metadata":          map[string]any{"managed_by": "identity"},
		}},
	})
	if err != nil {
		t.Fatalf("decode managed user: %v", err)
	}
	if len(users) != 1 || !users[0].ManagedByIdentity {
		t.Fatalf("managed metadata not preserved: %+v", users)
	}
}

func TestDecodeServerUsersRejectsManualManagedLabel(t *testing.T) {
	_, err := decodeServerTrojanUsers(map[string]any{
		serverUsersKey: []any{map[string]any{
			"user_label":        "idp-alice@xp2p.local",
			"active_credential": "secret",
		}},
	})
	if err == nil {
		t.Fatal("expected reserved label error")
	}
}
