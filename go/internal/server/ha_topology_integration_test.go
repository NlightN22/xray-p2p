package server

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/ha"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func TestHATopologyReplicationMaterializesEquivalentServerState(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	serverA := filepath.Join(root, "server-a.toml")
	serverB := filepath.Join(root, "server-b.toml")
	userRedirect := redirect.Rule{CIDR: "10.9.0.0/16", OutboundTag: "user-owned"}
	for _, path := range []string{serverA, serverB} {
		if err := writeServerStateDoc(path, map[string]any{
			serverReverseStateKey:  map[string]serverReverseChannel{"user.rev": {UserID: "user", Tag: "user.rev", Domain: "user.rev", Host: "user.rev"}},
			serverRedirectRulesKey: []redirect.Rule{userRedirect},
		}); err != nil {
			t.Fatal(err)
		}
	}
	generation := replicatedTopologyGeneration(t)
	receiverPeer := ha.Peer{ID: "coordinator", Secret: "shared"}
	receiver, err := ha.NewStoreWithLocalID("server-b", []ha.Peer{receiverPeer}, ha.Generation{})
	if err != nil {
		t.Fatal(err)
	}
	receiver.SetCommitter(func(candidate ha.Generation) error { return CommitHAGeneration(serverB, candidate) })
	server := httptest.NewTLSServer(ha.NewHTTPHandler(receiver))
	defer server.Close()
	remotePeer := ha.Peer{ID: "server-b", Endpoint: server.URL, Secret: "shared"}
	coordinator, err := ha.NewStoreWithLocalID("coordinator", []ha.Peer{remotePeer}, ha.Generation{})
	if err != nil {
		t.Fatal(err)
	}
	coordinator.SetCommitter(func(candidate ha.Generation) error { return CommitHAGeneration(serverA, candidate) })
	if err := (ha.Coordinator{Client: ha.SyncClient{HTTPClient: server.Client()}}).Sync(t.Context(), coordinator, generation); err != nil {
		t.Fatal(err)
	}
	assertEquivalentHATopologyState(t, serverA, generation, userRedirect)
	assertEquivalentHATopologyState(t, serverB, generation, userRedirect)
	state, err := identitysync.DefaultStore().Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Current == nil || state.Current.ID != "id-gen-2" || !state.Current.Subjects["alice"].Provisioned {
		t.Fatalf("identity state = %+v", state)
	}
}

func replicatedTopologyGeneration(t *testing.T) ha.Generation {
	t.Helper()
	redirects, err := json.Marshal([]redirect.Rule{{CIDR: "10.8.0.0/16", OutboundTag: "ha-rev"}, {Domain: "svc.example", OutboundTag: "ha-rev"}})
	if err != nil {
		t.Fatal(err)
	}
	identityGeneration := identitysync.Generation{
		ID:                 "id-gen-2",
		ProviderInstanceID: "idp",
		Subjects: map[string]identitysync.Subject{
			"alice": {ExternalSubject: "alice", UserLabel: "idp-alice@xp2p.local", Active: true},
		},
		Groups: map[string]identitysync.Group{"ops": {ID: "ops", DirectMembers: []string{"alice"}}},
	}
	identityPayload, err := json.Marshal(identityGeneration)
	if err != nil {
		t.Fatal(err)
	}
	provisioned, err := json.Marshal([]string{"idp-alice@xp2p.local"})
	if err != nil {
		t.Fatal(err)
	}
	return ha.Generation{
		Number: 2,
		Group: ha.Group{
			ID:       "group-1",
			Tag:      "ha-group",
			Selector: ha.Selector{Mode: "automatic", FailureThreshold: 2},
			Members: []ha.Member{
				{ID: "a", Tag: "server-a", Host: "a.example", Port: 443, Profile: "trojan-tls", Confirmed: true},
				{ID: "b", Tag: "server-b", Host: "b.example", Port: 443, Profile: "trojan-tls", Confirmed: true},
			},
		},
		Channels:    []ha.Channel{{ID: "rev", Tag: "ha-rev", Domain: "portal.example", UserID: "idp-alice@xp2p.local", Binding: ha.ChannelBinding{GroupTag: "ha-group"}}},
		Redirects:   redirects,
		IdentityACL: identityPayload,
		Provisioned: provisioned,
	}
}

func assertEquivalentHATopologyState(t *testing.T, path string, generation ha.Generation, userRedirect redirect.Rule) {
	t.Helper()
	doc, err := loadServerStateDoc(path)
	if err != nil {
		t.Fatal(err)
	}
	reverse, err := decodeServerReverseState(doc)
	if err != nil {
		t.Fatal(err)
	}
	channel, ok := reverse["ha-rev"]
	if !ok || channel.Domain != "portal.example" || channel.UserID != "idp-alice@xp2p.local" {
		t.Fatalf("reverse state %s = %+v", path, reverse)
	}
	if _, ok := reverse["user.rev"]; !ok {
		t.Fatalf("user-owned reverse was removed from %s: %+v", path, reverse)
	}
	redirects, err := decodeServerRedirectRules(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(redirects) != 3 || redirects[0].CIDR != userRedirect.CIDR {
		t.Fatalf("redirects %s = %+v", path, redirects)
	}
	stored, err := decodeHAGeneration(doc)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Number != generation.Number || stored.Group.Tag != generation.Group.Tag {
		t.Fatalf("generation %s = %+v", path, stored)
	}
}
