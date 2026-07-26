package ha

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func testGeneration(number uint64) Generation {
	return Generation{Number: number, Group: Group{ID: "g1", Tag: "group-main", Members: []Member{{ID: "a", Tag: "a", Host: "a.example", Port: 443, Profile: "trojan-tls", Confirmed: true}}}, Channels: []Channel{{ID: "c1", Tag: "reverse-c1", Domain: "portal.example", Binding: ChannelBinding{GroupTag: "group-main"}}}}
}

func TestPrepareAcknowledgeCommitRejectsOutOfOrderAndUnauthorizedPeers(t *testing.T) {
	peer := Peer{ID: "peer-a", Secret: "shared"}
	store, err := NewStore([]Peer{peer}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	candidate := testGeneration(2)
	signature, err := Sign(peer, candidate)
	if err != nil {
		t.Fatal(err)
	}
	ack := store.Prepare(PrepareRequest{PeerID: peer.ID, Generation: candidate, Signature: signature})
	if !ack.Ready {
		t.Fatalf("prepare rejected: %+v", ack)
	}
	if _, err := store.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := store.Committed().Number; got != 2 {
		t.Fatalf("committed = %d", got)
	}
	staleSignature, _ := Sign(peer, testGeneration(2))
	if ack := store.Prepare(PrepareRequest{PeerID: peer.ID, Generation: testGeneration(2), Signature: staleSignature}); ack.Ready {
		t.Fatal("stale generation was accepted")
	}
	if ack := store.Prepare(PrepareRequest{PeerID: "unknown", Generation: testGeneration(3)}); ack.Ready {
		t.Fatal("unknown peer was accepted")
	}
}

func TestChannelCannotFinalizeUntilDisabled(t *testing.T) {
	generation := testGeneration(1)
	if err := generation.CanFinalizeChannel("c1"); err != ErrChannelReferenced {
		t.Fatalf("finalize error = %v", err)
	}
	generation.Channels[0].Binding.Disabled = true
	if err := generation.CanFinalizeChannel("c1"); err != nil {
		t.Fatal(err)
	}
}

func TestCoordinatorCommitsStagedGenerationWithoutPeers(t *testing.T) {
	store, err := NewStore(nil, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := (Coordinator{}).Sync(t.Context(), store, testGeneration(2)); err != nil {
		t.Fatal(err)
	}
	if got := store.Committed().Number; got != 2 {
		t.Fatalf("committed generation = %d", got)
	}
}

func TestGenerationRejectsInvalidChannelBindingsAndMembers(t *testing.T) {
	generation := testGeneration(1)
	generation.Channels[0].Binding = ChannelBinding{GroupTag: "group-main", EndpointTag: "a"}
	if err := generation.Validate(); err == nil {
		t.Fatal("multiple channel bindings were accepted")
	}
	generation = testGeneration(1)
	generation.Channels[0].Binding = ChannelBinding{EndpointTag: "missing"}
	if err := generation.Validate(); err == nil {
		t.Fatal("unknown endpoint binding was accepted")
	}
	generation = testGeneration(1)
	generation.Group.Members[0].Tombstone = true
	if err := generation.Validate(); err == nil {
		t.Fatal("confirmed tombstone was accepted")
	}
}

func TestFinalizeUnknownChannelFails(t *testing.T) {
	if err := testGeneration(1).CanFinalizeChannel("missing"); err != ErrChannelNotFound {
		t.Fatalf("finalize error = %v", err)
	}
}

func TestCoordinatorReplicatesPreparedGeneration(t *testing.T) {
	peer := Peer{ID: "coordinator", Secret: "shared"}
	receiver, err := NewStoreWithLocalID("coordinator", []Peer{peer}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	var connections atomic.Int32
	server := httptest.NewUnstartedServer(NewHTTPHandler(receiver))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.StartTLS()
	defer server.Close()
	peer.Endpoint = server.URL
	coordinator, err := NewStoreWithLocalID("coordinator", []Peer{peer}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := (Coordinator{Client: SyncClient{HTTPClient: server.Client()}}).Sync(t.Context(), coordinator, testGeneration(2)); err != nil {
		t.Fatal(err)
	}
	if got := receiver.Committed().Number; got != 2 {
		t.Fatalf("receiver committed generation = %d", got)
	}
	if got := coordinator.Committed().Number; got != 2 {
		t.Fatalf("coordinator committed generation = %d", got)
	}
	if got := connections.Load(); got != 1 {
		t.Fatalf("prepare and commit opened %d connections, want 1", got)
	}
}

func TestCoordinatorReplicatesWithDistinctLocalAndRemotePeerIDs(t *testing.T) {
	receiverPeer := Peer{ID: "server-a", Secret: "shared"}
	receiver, err := NewStoreWithLocalID("server-b", []Peer{receiverPeer}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(NewHTTPHandler(receiver))
	defer server.Close()
	remotePeer := Peer{ID: "server-b", Endpoint: server.URL, Secret: "shared"}
	coordinator, err := NewStoreWithLocalID("server-a", []Peer{remotePeer}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := (Coordinator{Client: SyncClient{HTTPClient: server.Client()}}).Sync(t.Context(), coordinator, testGeneration(2)); err != nil {
		t.Fatal(err)
	}
	if got := receiver.Committed().Number; got != 2 {
		t.Fatalf("receiver committed generation = %d", got)
	}
}

func TestCoordinatorFailureDiscardsLocalCandidate(t *testing.T) {
	peer := Peer{ID: "peer", Endpoint: "https://127.0.0.1:1", Secret: "shared"}
	store, err := NewStoreWithLocalID("self", []Peer{peer}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := (Coordinator{}).Sync(t.Context(), store, testGeneration(2)); err == nil {
		t.Fatal("unreachable peer was accepted")
	}
	committed, pending, _ := store.Status()
	if committed.Number != 1 || pending != nil {
		t.Fatalf("unexpected state after failed synchronization: committed=%d pending=%+v", committed.Number, pending)
	}
}

func TestStoreRefreshUpdatesDurableStateOnlyWhenIdle(t *testing.T) {
	store, err := NewStore([]Peer{{ID: "old", Secret: "s"}}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Refresh([]Peer{{ID: "new", Secret: "s"}}, testGeneration(2)); err != nil {
		t.Fatal(err)
	}
	if got := store.Committed().Number; got != 2 {
		t.Fatalf("committed generation = %d", got)
	}
	if peers := store.Peers(); len(peers) != 1 || peers[0].ID != "new" {
		t.Fatalf("peers = %+v", peers)
	}
	if err := store.Stage(testGeneration(3)); err != nil {
		t.Fatal(err)
	}
	if err := store.Refresh([]Peer{{ID: "next", Secret: "s"}}, testGeneration(4)); err != nil {
		t.Fatal(err)
	}
	committed, pending, _ := store.Status()
	if committed.Number != 2 || pending == nil || pending.Number != 3 {
		t.Fatalf("committed=%d pending=%+v", committed.Number, pending)
	}
}
