package identitysync

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRecoverDiscardsUnconfirmedPendingGeneration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := StoreAt(path)
	pending := &Generation{
		ID:                 "next",
		ProviderInstanceID: "corp",
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		Subjects:           map[string]Subject{},
		Groups:             map[string]Group{},
	}
	candidateHash, err := HashJSON(pending)
	if err != nil {
		t.Fatalf("hash pending: %v", err)
	}
	if err := store.Save(State{
		Current: &Generation{ID: "current", Subjects: map[string]Subject{}, Groups: map[string]Group{}},
		Pending: pending,
		Transaction: &Transaction{
			CandidateGenerationID: "next",
			CandidateStateHash:    candidateHash,
		},
		Status: Status{State: SyncStatusSuccess},
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := store.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if state.Current.ID != "current" || state.Pending != nil || state.Transaction != nil || state.Status.State != SyncStatusError {
		t.Fatalf("unconfirmed transaction was not discarded: %+v", state)
	}
}

func TestStoreUpdateCurrentRefreshesStagedPendingCandidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := StoreAt(path)
	current := &Generation{
		ID:                 "next",
		ProviderInstanceID: "corp",
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		Subjects:           map[string]Subject{"u1": {ExternalSubject: "u1", UserLabel: "idp-one@xp2p.local"}},
		Groups:             map[string]Group{},
	}
	candidateHash, err := HashJSON(current)
	if err != nil {
		t.Fatalf("hash current: %v", err)
	}
	if err := store.Save(State{
		Current: current,
		Pending: current,
		Transaction: &Transaction{
			CandidateGenerationID: "next",
			CandidateStateHash:    candidateHash,
		},
		Status: Status{State: SyncStatusSuccess},
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := store.SetProvisionedByLabel("idp-one@xp2p.local", true); err != nil {
		t.Fatalf("set provisioned: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !state.Current.Subjects["u1"].Provisioned || !state.Pending.Subjects["u1"].Provisioned {
		t.Fatalf("staged pending candidate was not refreshed: %+v", state)
	}
	refreshedHash, err := HashJSON(state.Pending)
	if err != nil {
		t.Fatalf("hash pending: %v", err)
	}
	if state.Transaction.CandidateStateHash != refreshedHash {
		t.Fatalf("transaction candidate hash was not refreshed: got %s want %s", state.Transaction.CandidateStateHash, refreshedHash)
	}
}

func TestStoreRecoverPromotesConfirmedPendingGeneration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := StoreAt(path)
	pending := &Generation{
		ID:                 "next",
		ProviderInstanceID: "corp",
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		Subjects:           map[string]Subject{"u1": {ExternalSubject: "u1", UserLabel: "idp-one@xp2p.local"}},
		Groups:             map[string]Group{},
	}
	candidateHash, err := HashJSON(pending)
	if err != nil {
		t.Fatalf("hash pending: %v", err)
	}
	if err := store.Save(State{
		Current: &Generation{ID: "current", Subjects: map[string]Subject{}, Groups: map[string]Group{}},
		Pending: pending,
		Transaction: &Transaction{
			CandidateGenerationID: "next",
			CandidateStateHash:    candidateHash,
			CandidateDesiredHash:  "desired",
			CandidateLiveHash:     "live",
			RuntimeResult:         "applied",
		},
		Status: Status{State: SyncStatusSuccess},
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := store.RecoverCommitted("desired", "live"); err != nil {
		t.Fatalf("recover: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if state.Current.ID != "next" || state.Pending != nil || state.Transaction != nil || state.Status.State != SyncStatusSuccess {
		t.Fatalf("confirmed transaction was not promoted: %+v", state)
	}
}

func TestStoreRecoverRejectsCommittedHashMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := StoreAt(path)
	pending := &Generation{
		ID:                 "next",
		ProviderInstanceID: "corp",
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		Subjects:           map[string]Subject{"u1": {ExternalSubject: "u1", UserLabel: "idp-one@xp2p.local"}},
		Groups:             map[string]Group{},
	}
	candidateHash, err := HashJSON(pending)
	if err != nil {
		t.Fatalf("hash pending: %v", err)
	}
	if err := store.Save(State{
		Current: &Generation{ID: "current", Subjects: map[string]Subject{}, Groups: map[string]Group{}},
		Pending: pending,
		Transaction: &Transaction{
			CandidateGenerationID: "next",
			CandidateStateHash:    candidateHash,
			CandidateDesiredHash:  "desired",
			CandidateLiveHash:     "live",
			RuntimeResult:         "applied",
		},
		Status: Status{State: SyncStatusSuccess},
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := store.RecoverCommitted("desired", "other-live"); err != nil {
		t.Fatalf("recover: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if state.Current.ID != "current" || state.Pending != nil || state.Transaction != nil || state.Status.State != SyncStatusError {
		t.Fatalf("hash mismatch was not discarded: %+v", state)
	}
}
