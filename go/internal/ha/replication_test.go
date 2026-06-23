package ha

import "testing"

func testGeneration(number uint64) Generation {
	return Generation{Number: number, Group: Group{ID: "g1", Tag: "group-main", Members: []Member{{ID: "a", Tag: "a", Confirmed: true}}}, Channels: []Channel{{ID: "c1", Tag: "reverse-c1", Domain: "portal.example", Binding: ChannelBinding{GroupTag: "group-main"}}}}
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
