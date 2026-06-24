package ha

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestElectionUsesStableLowestVotingPeer(t *testing.T) {
	election := ElectCoordinator("server-b", []Peer{
		{ID: "witness", Secret: "s", Witness: true},
		{ID: "server-a", Secret: "s"},
		{ID: "observer", Secret: "s", NonVoting: true},
	})
	if election.Coordinator != "server-a" {
		t.Fatalf("coordinator = %q", election.Coordinator)
	}
	if election.Quorum != 2 {
		t.Fatalf("quorum = %d", election.Quorum)
	}
	if len(election.Voters) != 3 {
		t.Fatalf("voters = %+v", election.Voters)
	}
}

func TestQuorumLossKeepsCommittedGeneration(t *testing.T) {
	store, err := NewStoreWithLocalID("server-a", []Peer{{ID: "server-b", Secret: "s"}}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Stage(testGeneration(2)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Commit(); !errors.Is(err, ErrQuorumUnavailable) {
		t.Fatalf("commit error = %v", err)
	}
	if got := store.Committed().Number; got != 1 {
		t.Fatalf("committed generation = %d", got)
	}
}

func TestWitnessAllowsReplacementCommitWithMajority(t *testing.T) {
	witness, err := NewStoreWithLocalID("witness", []Peer{{ID: "server-a", Secret: "s"}}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(NewHTTPHandler(witness))
	defer server.Close()

	store, err := NewStoreWithLocalID("server-a", []Peer{
		{ID: "server-b", Endpoint: "https://127.0.0.1:1", Secret: "s"},
		{ID: "witness", Endpoint: server.URL, Secret: "s", Witness: true},
	}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	candidate := testGeneration(2)
	candidate.Group.Members = append(candidate.Group.Members, Member{ID: "replacement", Tag: "replacement", Host: "replacement.example", Port: 443, Profile: "trojan-tls", Confirmed: true})
	if err := (Coordinator{Client: SyncClient{HTTPClient: server.Client()}}).Sync(t.Context(), store, candidate); err != nil {
		t.Fatal(err)
	}
	if got := store.Committed().Number; got != 2 {
		t.Fatalf("local committed generation = %d", got)
	}
	if got := witness.Committed().Number; got != 2 {
		t.Fatalf("witness committed generation = %d", got)
	}
}

func TestConflictingWritesRejectOutOfOrderGeneration(t *testing.T) {
	store, err := NewStoreWithLocalID("server-a", []Peer{{ID: "server-b", Secret: "s"}}, testGeneration(2))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Stage(testGeneration(2)); !errors.Is(err, ErrGenerationOutOfOrder) {
		t.Fatalf("stage error = %v", err)
	}
}

func TestTombstoneRequiresExplicitFinalization(t *testing.T) {
	generation := testGeneration(1)
	generation.Group.Members = append(generation.Group.Members, Member{ID: "old", Tag: "old", Tombstone: true})
	generation.Tombstones = []string{"old"}
	if err := generation.Validate(); err != nil {
		t.Fatal(err)
	}
	generation.Group.Members[1].Confirmed = true
	if err := generation.Validate(); err == nil {
		t.Fatal("confirmed tombstone was accepted")
	}
}

func TestForceReconfigurationRequiresAuthorization(t *testing.T) {
	store, err := NewStoreWithLocalID("server-a", []Peer{{ID: "server-b", Secret: "s"}}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ForceReconfigure(testGeneration(2), ForceReconfiguration{Authorization: "wrong", Reason: "server-b lost"}); !errors.Is(err, ErrForceReconfigureForbidden) {
		t.Fatalf("force error = %v", err)
	}
	committed, err := store.ForceReconfigure(testGeneration(2), ForceReconfiguration{Authorization: "force", Reason: "server-b lost"})
	if err != nil {
		t.Fatal(err)
	}
	if committed.Number != 2 {
		t.Fatalf("committed generation = %d", committed.Number)
	}
}

func TestHTTPStatusExposesRecoveryDiagnostics(t *testing.T) {
	store, err := NewStoreWithLocalID("server-a", []Peer{{ID: "witness", Secret: "s", Witness: true}}, testGeneration(1))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHTTPHandler(store))
	defer server.Close()
	response, err := server.Client().Get(server.URL + PathStatus)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %s", response.Status)
	}
}
