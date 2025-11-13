package installstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/version"
)

// Kind represents the role of an installation tracked by a state file.
type Kind string

const (
	// KindServer marks a server installation.
	KindServer Kind = "server"
	// KindClient marks a client installation.
	KindClient Kind = "client"
	// FileName is retained for legacy single-role installations.
	FileName = "install-state.json"
)

// FileNameForKind returns the canonical marker name for the provided kind.
func FileNameForKind(kind Kind) string {
	switch kind {
	case KindClient:
		return layout.ClientStateFileName
	case KindServer:
		return layout.ServerStateFileName
	default:
		return FileName
	}
}

// ErrRoleNotInstalled is returned when the state file exists but lacks the requested role.
var ErrRoleNotInstalled = errors.New("installstate: role marker not found")

// Marker captures the installation metadata persisted on disk.
type Marker struct {
	Kind        Kind      `json:"kind"`
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
}

// Write stores or updates the installation marker for the provided kind.
func Write(path string, kind Kind) error {
	if err := validateKind(kind); err != nil {
		return err
	}

	state, err := loadState(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		state = fileState{Roles: make(map[Kind]Marker)}
	}

	state.Roles[kind] = Marker{
		Kind:        kind,
		Version:     version.Current(),
		InstalledAt: time.Now().UTC(),
	}

	return state.save(path)
}

// Read loads the installation marker for the requested role from disk.
func Read(path string, kind Kind) (Marker, error) {
	if err := validateKind(kind); err != nil {
		return Marker{}, err
	}

	state, err := loadState(path)
	if err != nil {
		return Marker{}, err
	}

	marker, ok := state.Roles[kind]
	if !ok {
		return Marker{}, ErrRoleNotInstalled
	}
	marker.Kind = kind
	return marker, nil
}

// Remove deletes the role marker from disk. When the last role is removed the file is deleted.
func Remove(path string, kind Kind) error {
	if err := validateKind(kind); err != nil {
		return err
	}

	state, err := loadState(path)
	if err != nil {
		return err
	}

	if _, ok := state.Roles[kind]; !ok {
		return ErrRoleNotInstalled
	}

	delete(state.Roles, kind)
	if len(state.Roles) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("installstate: remove marker: %w", err)
		}
		return nil
	}
	return state.save(path)
}

// Roles returns a copy of all role markers stored in the install-state file.
func Roles(path string) (map[Kind]Marker, error) {
	state, err := loadState(path)
	if err != nil {
		return nil, err
	}

	result := make(map[Kind]Marker, len(state.Roles))
	for kind, marker := range state.Roles {
		marker.Kind = kind
		result[kind] = marker
	}
	return result, nil
}

// HasValidMarker returns true when a state file exists at path and contains the expected kind.
func HasValidMarker(path string, kind Kind) (bool, error) {
	_, err := Read(path, kind)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrRoleNotInstalled) {
		return false, nil
	}
	return false, err
}

func validateKind(kind Kind) error {
	switch kind {
	case KindServer, KindClient:
		return nil
	default:
		return fmt.Errorf("unknown marker kind %q", kind)
	}
}

type fileState struct {
	Roles map[Kind]Marker `json:"roles"`
}

func (s fileState) save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("installstate: ensure directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("installstate: encode marker: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("installstate: write marker: %w", err)
	}
	return nil
}

func loadState(path string) (fileState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fileState{}, err
	}

	var multi fileState
	if err := json.Unmarshal(data, &multi); err == nil && len(multi.Roles) > 0 {
		normalizeKinds(multi.Roles)
		return multi, nil
	}

	var legacy Marker
	if err := json.Unmarshal(data, &legacy); err != nil {
		return fileState{}, fmt.Errorf("installstate: decode %s: %w", path, err)
	}
	if err := validateKind(legacy.Kind); err != nil {
		return fileState{}, fmt.Errorf("installstate: %w", err)
	}
	return fileState{
		Roles: map[Kind]Marker{
			legacy.Kind: legacy,
		},
	}, nil
}

func normalizeKinds(roles map[Kind]Marker) {
	for kind, marker := range roles {
		marker.Kind = kind
		roles[kind] = marker
	}
}
