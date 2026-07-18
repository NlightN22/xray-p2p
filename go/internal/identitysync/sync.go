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
	var status Status
	err := DefaultOperationLock().With(ctx, func() error {
		var syncErr error
		status, syncErr = s.prepareLocked(ctx, provider)
		if syncErr != nil || status.State != SyncStatusSuccess {
			return syncErr
		}
		return s.promoteLocked()
	})
	return status, err
}

func (s Service) SyncAndApply(ctx context.Context, provider ProviderRef, apply func(context.Context) (string, error)) (Status, string, error) {
	var status Status
	result := ""
	err := DefaultOperationLock().With(ctx, func() error {
		var syncErr error
		status, syncErr = s.prepareLocked(ctx, provider)
		if syncErr != nil || status.State != SyncStatusSuccess {
			return syncErr
		}
		if apply == nil {
			return s.promoteLocked()
		}
		var applyErr error
		result, applyErr = apply(ctx)
		if applyErr != nil {
			_ = s.abortPendingLocked(applyErr.Error())
			return applyErr
		}
		if result == "staged" {
			return s.stageLocked()
		}
		return s.promoteLocked()
	})
	return status, result, err
}

func (s Service) prepareLocked(ctx context.Context, provider ProviderRef) (Status, error) {
	if err := provider.Validate(); err != nil {
		return Status{State: SyncStatusError, Error: err.Error()}, err
	}
	if s.Fetcher == nil {
		err := fmt.Errorf("identity snapshot fetcher is not configured")
		return Status{State: SyncStatusError, Error: err.Error()}, err
	}
	store := s.store()
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
	previousID := ""
	if state.Current != nil {
		previousID = state.Current.ID
	}
	previousHash, err := HashJSON(state.Current)
	if err != nil {
		return Status{State: SyncStatusError, Error: err.Error()}, err
	}
	candidateHash, err := HashJSON(next)
	if err != nil {
		return Status{State: SyncStatusError, Error: err.Error()}, err
	}
	state.Provider = &provider
	state.Pending = next
	state.Transaction = &Transaction{
		PreviousGenerationID:  previousID,
		CandidateGenerationID: next.ID,
		StartedAt:             nowUTCString(nowUTC()),
		PreviousStateHash:     previousHash,
		CandidateStateHash:    candidateHash,
	}
	state.Status = status
	if err := store.Save(state); err != nil {
		return Status{State: SyncStatusError, Error: err.Error()}, err
	}
	return status, nil
}

func (s Service) promoteLocked() error {
	store := s.store()
	state, err := store.Load()
	if err != nil {
		return err
	}
	if state.Pending == nil || state.Transaction == nil {
		return fmt.Errorf("identity pending generation is not available")
	}
	if state.Transaction.CandidateGenerationID != state.Pending.ID {
		return fmt.Errorf("identity pending generation does not match transaction")
	}
	state.Current = state.Pending
	state.Pending = nil
	state.Transaction = nil
	state.Status.State = SyncStatusSuccess
	state.Status.Error = ""
	return store.Save(state)
}

func (s Service) stageLocked() error {
	store := s.store()
	state, err := store.Load()
	if err != nil {
		return err
	}
	if state.Pending == nil || state.Transaction == nil {
		return fmt.Errorf("identity pending generation is not available")
	}
	if state.Transaction.CandidateGenerationID != state.Pending.ID {
		return fmt.Errorf("identity pending generation does not match transaction")
	}
	state.Current = state.Pending
	state.Status.State = SyncStatusSuccess
	state.Status.Error = ""
	return store.Save(state)
}

func (s Service) abortPendingLocked(reason string) error {
	store := s.store()
	state, err := store.Load()
	if err != nil {
		return err
	}
	state.Pending = nil
	state.Transaction = nil
	state.Status.State = SyncStatusError
	state.Status.Error = reason
	return store.Save(state)
}

func (s Service) store() Store {
	if s.Store.path == "" {
		return DefaultStore()
	}
	return s.Store
}
