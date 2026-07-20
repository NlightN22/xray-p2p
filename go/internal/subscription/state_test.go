package subscription

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestReconcileIsSourceOwned(t *testing.T) {
	old := Snapshot{Source: SourceRef{ID: "source-a"}, Offers: []ConnectionOffer{{StableID: "keep", Credential: "old"}, {StableID: "remove"}}}
	next := Snapshot{Source: SourceRef{ID: "source-a"}, Offers: []ConnectionOffer{{StableID: "keep", Credential: "new"}, {StableID: "add"}}}
	result := Reconcile(&old, next)
	if len(result.Added) != 1 || len(result.Updated) != 1 || len(result.Removed) != 1 {
		t.Fatalf("unexpected reconcile: %+v", result)
	}
	other := Snapshot{Source: SourceRef{ID: "source-b"}, Offers: []ConnectionOffer{{StableID: "other"}}}
	result = Reconcile(&old, other)
	if len(result.Removed) != 0 || len(result.Added) != 1 {
		t.Fatalf("cross-source reconcile removed offers: %+v", result)
	}
}

func TestStoreRejectsConcurrentWriteAndKeepsSecretPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subscriptions", "source.json")
	store := Store{Path: path}
	_, digest, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	state := PersistedSource{SourceRef: SourceRef{ID: "source-a"}, URL: "https://example.test/secret"}
	if err := store.Save(state, digest); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(state, digest); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("concurrent save error = %v", err)
	}
}
