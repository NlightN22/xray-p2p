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

func (s Store) DetachProvider() error {
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
