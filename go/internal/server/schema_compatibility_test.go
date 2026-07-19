//go:build windows || linux

package server

import (
	"path/filepath"
	"testing"
)

func TestSchemaCompatibilityV026ServerDesired(t *testing.T) {
	path := filepath.Join("..", "..", "..", "tests", "schema", "compat", "v0.2.6", "xp2p-server.toml")
	doc, err := loadServerStateDoc(path)
	if err != nil {
		t.Fatalf("load v0.2.6 server Desired: %v", err)
	}
	users, err := decodeServerTrojanUsers(doc)
	if err != nil {
		t.Fatalf("decode v0.2.6 users: %v", err)
	}
	if len(users) != 1 || users[0].Email != "alice" {
		t.Fatalf("users = %#v", users)
	}
	if doc[serverRedirectRulesKey] == nil || doc[serverReverseStateKey] == nil || doc[serverForwardRulesKey] == nil {
		t.Fatalf("legacy server state was not preserved: %#v", doc)
	}
}
