package identitysync

import (
	"context"
	"fmt"
)

type Fetcher interface {
	FetchSnapshot(context.Context, ProviderRef) (Snapshot, error)
}

type Service struct {
	Store    Store
	Fetcher  Fetcher
	Allocate LabelAllocator
	Now      func() string
}

func (s Service) Sync(ctx context.Context, provider ProviderRef) (Status, error) {
	if err := provider.Validate(); err != nil {
		return Status{State: SyncStatusError, Error: err.Error()}, err
	}
	if s.Fetcher == nil {
		err := fmt.Errorf("identity snapshot fetcher is not configured")
		return Status{State: SyncStatusError, Error: err.Error()}, err
	}
	store := s.Store
	if store.path == "" {
		store = DefaultStore()
	}
	state, err := store.Load()
	if err != nil {
		return Status{State: SyncStatusError, Error: err.Error()}, err
	}
	snapshot, err := s.Fetcher.FetchSnapshot(ctx, provider)
	if err != nil {
		state.Status = Status{State: SyncStatusError, Error: err.Error()}
		_ = store.Save(state)
		return state.Status, err
	}
	snapshot.Provider = provider
	next, status, err := ApplySnapshot(state.Current, snapshot, nowUTC(), s.Allocate)
	if err != nil {
		state.Status = status
		_ = store.Save(state)
		return status, err
	}
	if status.State != SyncStatusSuccess {
		state.Status = status
		_ = store.Save(state)
		return status, nil
	}
	state.Provider = &provider
	state.Pending = nil
	state.Current = next
	state.Status = status
	if err := store.Save(state); err != nil {
		return Status{State: SyncStatusError, Error: err.Error()}, err
	}
	return status, nil
}
