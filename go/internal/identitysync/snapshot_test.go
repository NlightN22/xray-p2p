package identitysync

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestApplySnapshotAllocatesStableLabelsAndDeletesOnlyOnFullSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 23, 1, 2, 3, 0, time.UTC)
	labels := labelSequence("idp-first@xp2p.local", "idp-second@xp2p.local")
	first, status, err := ApplySnapshot(nil, Snapshot{
		Provider: provider(),
		Complete: true,
		Subjects: []SnapshotSubject{
			{ExternalSubject: "u1", DisplayName: "User One"},
			{ExternalSubject: "u2", DisplayName: "User Two"},
		},
	}, now, labels)
	if err != nil {
		t.Fatalf("apply first snapshot: %v", err)
	}
	if status.State != SyncStatusSuccess {
		t.Fatalf("unexpected status: %s", status.State)
	}
	if first.Subjects["u1"].UserLabel != "idp-first@xp2p.local" {
		t.Fatalf("unexpected label: %+v", first.Subjects["u1"])
	}

	partial, status, err := ApplySnapshot(first, Snapshot{
		Provider: provider(),
		Complete: false,
		Subjects: []SnapshotSubject{{ExternalSubject: "u1"}},
	}, now, labels)
	if err != nil {
		t.Fatalf("partial snapshot must not fail hard: %v", err)
	}
	if status.State != SyncStatusPartial {
		t.Fatalf("unexpected partial status: %s", status.State)
	}
	if len(partial.Subjects) != 2 {
		t.Fatalf("partial snapshot removed subjects: %+v", partial.Subjects)
	}

	second, _, err := ApplySnapshot(first, Snapshot{
		Provider: provider(),
		Complete: true,
		Subjects: []SnapshotSubject{{ExternalSubject: "u1", DisplayName: "User One"}},
	}, now, labels)
	if err != nil {
		t.Fatalf("apply second snapshot: %v", err)
	}
	if _, ok := second.Subjects["u2"]; ok {
		t.Fatalf("full snapshot must remove unreachable subject u2")
	}
	if second.Subjects["u1"].UserLabel != "idp-first@xp2p.local" {
		t.Fatalf("existing label changed: %+v", second.Subjects["u1"])
	}
}

func TestApplySnapshotResolvesNestedGroupsAndDNMembers(t *testing.T) {
	next, _, err := ApplySnapshot(nil, Snapshot{
		Provider: ProviderRef{InstanceID: "corp", Kind: ProviderLDAP, Scope: []string{"admins"}},
		Complete: true,
		Subjects: []SnapshotSubject{
			{ExternalSubject: "u1", DN: "CN=One,OU=Users,DC=example,DC=local"},
			{ExternalSubject: "u2", DN: "CN=Two,OU=Users,DC=example,DC=local"},
		},
		Groups: []SnapshotGroup{
			{ID: "admins", MemberDNs: []string{"cn=devs,ou=groups,dc=example,dc=local"}},
			{ID: "devs", DN: "CN=Devs,OU=Groups,DC=example,DC=local", MemberSubjects: []string{"u1"}, MemberDNs: []string{"cn=two,ou=users,dc=example,dc=local"}},
			{ID: "ignored", MemberSubjects: []string{"u2"}},
		},
	}, time.Now(), labelSequence("idp-one@xp2p.local", "idp-two@xp2p.local"))
	if err != nil {
		t.Fatalf("apply snapshot: %v", err)
	}
	if _, ok := next.Groups["ignored"]; ok {
		t.Fatalf("out-of-scope group was retained")
	}
	if _, ok := next.Subjects["u1"]; !ok {
		t.Fatalf("direct nested subject missing")
	}
	if _, ok := next.Subjects["u2"]; !ok {
		t.Fatalf("DN nested subject missing")
	}
	if got := next.Groups["admins"].DirectGroups; len(got) != 1 || got[0] != "devs" {
		t.Fatalf("nested group not resolved: %+v", got)
	}
}

func TestApplySnapshotDefaultLabelsAreDeterministic(t *testing.T) {
	now := time.Date(2026, 6, 23, 1, 2, 3, 0, time.UTC)
	snapshot := Snapshot{
		Provider: provider(),
		Complete: true,
		Subjects: []SnapshotSubject{{ExternalSubject: "stable-user"}},
	}
	first, _, err := ApplySnapshot(nil, snapshot, now, nil)
	if err != nil {
		t.Fatalf("apply first snapshot: %v", err)
	}
	second, _, err := ApplySnapshot(nil, snapshot, now, nil)
	if err != nil {
		t.Fatalf("apply second snapshot: %v", err)
	}
	if first.Subjects["stable-user"].UserLabel != second.Subjects["stable-user"].UserLabel {
		t.Fatalf("deterministic labels differ: %q != %q", first.Subjects["stable-user"].UserLabel, second.Subjects["stable-user"].UserLabel)
	}
	otherProvider := snapshot
	otherProvider.Provider.InstanceID = "other"
	other, _, err := ApplySnapshot(nil, otherProvider, now, nil)
	if err != nil {
		t.Fatalf("apply other provider snapshot: %v", err)
	}
	if first.Subjects["stable-user"].UserLabel == other.Subjects["stable-user"].UserLabel {
		t.Fatalf("provider instance id did not affect label: %q", first.Subjects["stable-user"].UserLabel)
	}
}

func TestApplySnapshotRejectsCycles(t *testing.T) {
	_, _, err := ApplySnapshot(nil, Snapshot{
		Provider: provider(),
		Complete: true,
		Groups: []SnapshotGroup{
			{ID: "g1", MemberGroups: []string{"g2"}},
			{ID: "g2", MemberGroups: []string{"g1"}},
		},
	}, time.Now(), labelSequence())
	if err == nil {
		t.Fatalf("expected cycle rejection")
	}
}

func TestServiceSyncAndProviderDetach(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := StoreAt(path)
	service := Service{
		Store: store,
		Fetcher: staticFetcher{snapshot: Snapshot{
			Complete: true,
			Subjects: []SnapshotSubject{{ExternalSubject: "u1"}},
		}},
		Allocate: labelSequence("idp-one@xp2p.local"),
	}
	status, err := service.Sync(context.Background(), provider())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if status.State != SyncStatusSuccess {
		t.Fatalf("unexpected status: %s", status.State)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if state.Current == nil || state.Current.Subjects["u1"].UserLabel == "" {
		t.Fatalf("current generation not saved: %+v", state)
	}
	if err := store.DetachProvider(); err != nil {
		t.Fatalf("detach: %v", err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatalf("load detached: %v", err)
	}
	if state.Provider != nil || state.Current == nil || !state.Current.Detached {
		t.Fatalf("provider was not detached: %+v", state)
	}
	if err := store.SelectProvider(provider()); err != nil {
		t.Fatalf("select provider: %v", err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatalf("load reattached: %v", err)
	}
	if state.Provider == nil || state.Current.Detached {
		t.Fatalf("provider was not reattached: %+v", state)
	}
	other := ProviderRef{InstanceID: "other", Kind: ProviderSCIM}
	if err := store.SelectProvider(other); err != nil {
		t.Fatalf("select replacement provider: %v", err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatalf("load replacement: %v", err)
	}
	if state.Provider == nil || state.Provider.InstanceID != "other" || !state.Current.Detached {
		t.Fatalf("provider replacement did not detach old cache: %+v", state)
	}
}

func TestServiceSyncAndApplyKeepsCurrentWhenRuntimeApplyFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := StoreAt(path)
	current := &Generation{
		ID:                 "current",
		ProviderInstanceID: "corp",
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		Subjects: map[string]Subject{
			"u1": {ExternalSubject: "u1", UserLabel: "idp-one@xp2p.local", Active: true},
		},
		Groups: map[string]Group{},
	}
	if err := store.Save(State{Current: current, Status: Status{State: SyncStatusSuccess, LastSuccess: time.Now().UTC().Format(time.RFC3339)}}); err != nil {
		t.Fatalf("save current: %v", err)
	}
	service := Service{
		Store: store,
		Fetcher: staticFetcher{snapshot: Snapshot{
			Complete: true,
			Subjects: []SnapshotSubject{{ExternalSubject: "u2"}},
		}},
		Allocate: labelSequence("idp-two@xp2p.local"),
	}
	_, _, err := service.SyncAndApply(context.Background(), provider(), func(context.Context) (string, error) {
		return "", errors.New("runtime apply failed")
	})
	if err == nil {
		t.Fatalf("expected runtime apply error")
	}
	state, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if state.Current == nil || state.Current.ID != "current" {
		t.Fatalf("current generation changed: %+v", state.Current)
	}
	if state.Pending != nil || state.Transaction != nil {
		t.Fatalf("pending transaction was not cleared: %+v", state)
	}
}

func TestServiceSyncAndApplyPromotesStagedCandidateForNextStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := StoreAt(path)
	service := Service{
		Store: store,
		Fetcher: staticFetcher{snapshot: Snapshot{
			Complete: true,
			Subjects: []SnapshotSubject{{ExternalSubject: "u1"}},
		}},
		Allocate: labelSequence("idp-one@xp2p.local"),
	}
	_, result, err := service.SyncAndApply(context.Background(), provider(), func(context.Context) (string, error) {
		return "staged", nil
	})
	if err != nil {
		t.Fatalf("sync and stage: %v", err)
	}
	if result != "staged" {
		t.Fatalf("unexpected apply result: %s", result)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if state.Current == nil || state.Pending != nil || state.Transaction != nil {
		t.Fatalf("staged candidate was not promoted for next start: %+v", state)
	}
}

type staticFetcher struct {
	snapshot Snapshot
	err      error
}

func (f staticFetcher) FetchSnapshot(context.Context, ProviderRef) (Snapshot, error) {
	return f.snapshot, f.err
}

func provider() ProviderRef {
	return ProviderRef{InstanceID: "corp", Kind: ProviderSCIM}
}

func labelSequence(labels ...string) LabelAllocator {
	index := 0
	return func(ProviderRef, string) (string, error) {
		if index >= len(labels) {
			return "idp-extra@xp2p.local", nil
		}
		label := labels[index]
		index++
		return label, nil
	}
}
