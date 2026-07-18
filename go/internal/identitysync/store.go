package identitysync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
)

type Store struct {
	path string
}

func DefaultStore() Store {
	return Store{path: config.IdentityStatePath()}
}

func StoreAt(path string) Store {
	return Store{path: filepath.Clean(path)}
}

func (s Store) IsZero() bool {
	return strings.TrimSpace(s.path) == ""
}

func (s Store) Load() (State, error) {
	if strings.TrimSpace(s.path) == "" {
		return State{}, errors.New("identity state path is empty")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewState(nowUTC()), nil
		}
		return State{}, fmt.Errorf("read identity state %s: %w", s.path, err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("parse identity state %s: %w", s.path, err)
	}
	if state.SchemaVersion == 0 {
		state.SchemaVersion = SchemaVersion
	}
	if state.Status.State == "" {
		state.Status.State = SyncStatusNever
	}
	return state, nil
}

func (s Store) Recover() error {
	return s.recoverWithHashes("", "")
}

func (s Store) RecoverCommitted(desiredHash, liveHash string) error {
	return s.recoverWithHashes(desiredHash, liveHash)
}

func (s Store) recoverWithHashes(desiredHash, liveHash string) error {
	state, err := s.Load()
	if err != nil {
		return err
	}
	if state.Transaction == nil || state.Pending == nil {
		return nil
	}
	if transactionPromotable(state.Transaction, state.Pending, desiredHash, liveHash) {
		state.Current = state.Pending
		state.Pending = nil
		state.Transaction = nil
		state.Status.State = SyncStatusSuccess
		state.Status.Error = ""
		return s.Save(state)
	}
	state.Pending = nil
	state.Transaction = nil
	state.Status.State = SyncStatusError
	state.Status.Error = "identity transaction recovery discarded unconfirmed pending generation"
	return s.Save(state)
}

func transactionPromotable(tx *Transaction, pending *Generation, desiredHash, liveHash string) bool {
	if tx == nil || pending == nil || tx.CandidateGenerationID != pending.ID {
		return false
	}
	candidateHash, err := HashJSON(pending)
	if err != nil || tx.CandidateStateHash != candidateHash {
		return false
	}
	if tx.RuntimeResult != "applied" && tx.RuntimeResult != "noop" && tx.RuntimeResult != "staged" {
		return false
	}
	if tx.CandidateDesiredHash == "" || tx.CandidateLiveHash == "" {
		return false
	}
	if desiredHash == "" || liveHash == "" {
		return false
	}
	if tx.CandidateDesiredHash != desiredHash || tx.CandidateLiveHash != liveHash {
		return false
	}
	return true
}

func (s Store) Save(state State) error {
	if strings.TrimSpace(s.path) == "" {
		return errors.New("identity state path is empty")
	}
	state.SchemaVersion = SchemaVersion
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode identity state: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("ensure identity state dir %s: %w", filepath.Dir(s.path), err)
	}
	if err := configio.WriteBytes(s.path, data, configio.WriteOptions{
		AuditPath: config.AuditLogPath(),
	}); err != nil {
		return err
	}
	return nil
}

func (s Store) UpdateTransaction(update func(*Transaction) error) error {
	state, err := s.Load()
	if err != nil {
		return err
	}
	if state.Transaction == nil {
		return errors.New("identity transaction is not available")
	}
	if err := update(state.Transaction); err != nil {
		return err
	}
	return s.Save(state)
}

func (s Store) DetachProvider() error {
	if err := s.Recover(); err != nil {
		return err
	}
	state, err := s.Load()
	if err != nil {
		return err
	}
	state.Provider = nil
	markDetached(state.Current)
	markDetached(state.Pending)
	state.Status.State = SyncStatusDetached
	state.Status.Error = ""
	return s.Save(state)
}

func (s Store) SelectProvider(provider ProviderRef) error {
	if err := provider.Validate(); err != nil {
		return err
	}
	if err := s.Recover(); err != nil {
		return err
	}
	state, err := s.Load()
	if err != nil {
		return err
	}
	state.Provider = &provider
	reattachGeneration(state.Current, provider.InstanceID)
	reattachGeneration(state.Pending, provider.InstanceID)
	if state.Current != nil && !state.Current.Detached {
		state.Status.State = SyncStatusSuccess
	} else {
		state.Status.State = SyncStatusNever
	}
	state.Status.Error = ""
	return s.Save(state)
}

func (s Store) UpdateCurrent(update func(*Generation) error) error {
	state, err := s.Load()
	if err != nil {
		return err
	}
	if state.Current == nil {
		return errors.New("identity current generation is not available")
	}
	if err := update(state.Current); err != nil {
		return err
	}
	if state.Pending != nil && state.Transaction != nil && state.Pending.ID == state.Current.ID && state.Transaction.CandidateGenerationID == state.Current.ID {
		state.Pending = state.Current
		candidateHash, err := HashJSON(state.Current)
		if err != nil {
			return err
		}
		state.Transaction.CandidateStateHash = candidateHash
	}
	return s.Save(state)
}

func (s Store) SetProvisionedByLabel(label string, provisioned bool) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return errors.New("identity user label is required")
	}
	return s.UpdateCurrent(func(current *Generation) error {
		for id, subject := range current.Subjects {
			if !strings.EqualFold(subject.UserLabel, label) {
				continue
			}
			subject.Provisioned = provisioned
			current.Subjects[id] = subject
			return nil
		}
		return fmt.Errorf("identity label %s not found", label)
	})
}

func (s Store) RemoveSubjectByLabel(label string) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return errors.New("identity user label is required")
	}
	return s.UpdateCurrent(func(current *Generation) error {
		for id, subject := range current.Subjects {
			if strings.EqualFold(subject.UserLabel, label) {
				delete(current.Subjects, id)
				return nil
			}
		}
		return nil
	})
}

func markDetached(generation *Generation) {
	if generation != nil {
		generation.Detached = true
	}
}

func reattachGeneration(generation *Generation, providerID string) {
	if generation == nil || strings.TrimSpace(providerID) == "" {
		return
	}
	if generation.ProviderInstanceID == providerID {
		generation.Detached = false
		return
	}
	generation.Detached = true
}
