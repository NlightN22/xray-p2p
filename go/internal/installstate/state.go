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
	// FileName is the default file name for state markers.
	FileName = layout.StateFileName
)

// Marker captures the installation metadata persisted on disk.
type Marker struct {
	Kind        Kind      `json:"kind"`
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
}

// Write stores the installation marker for the provided kind.
func Write(path string, kind Kind) error {
	if err := validateKind(kind); err != nil {
		return err
	}

	marker := Marker{
		Kind:        kind,
		Version:     version.Current(),
		InstalledAt: time.Now().UTC(),
	}

	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Errorf("installstate: encode marker: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("installstate: ensure directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("installstate: write marker: %w", err)
	}

	return nil
}

// Read loads the installation marker from disk.
func Read(path string) (Marker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Marker{}, err
	}

	var marker Marker
	if err := json.Unmarshal(data, &marker); err != nil {
		return Marker{}, fmt.Errorf("installstate: decode %s: %w", path, err)
	}

	if err := validateKind(marker.Kind); err != nil {
		return Marker{}, fmt.Errorf("installstate: %w", err)
	}

	return marker, nil
}

// HasValidMarker returns true when a state file exists at path and matches the expected kind.
func HasValidMarker(path string, kind Kind) (bool, error) {
	marker, err := Read(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	if marker.Kind != kind {
		return false, fmt.Errorf("installstate: marker kind %q does not match expected %q", marker.Kind, kind)
	}

	return true, nil
}

func validateKind(kind Kind) error {
	switch kind {
	case KindServer, KindClient:
		return nil
	default:
		return fmt.Errorf("unknown marker kind %q", kind)
	}
}
