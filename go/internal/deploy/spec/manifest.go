package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var (
	ErrRemoteHostEmpty = errors.New("xp2p: deployment manifest requires remote_host")
	ErrVersionEmpty    = errors.New("xp2p: deployment manifest requires xp2p_version")
	ErrGeneratedZero   = errors.New("xp2p: deployment manifest requires generated_at timestamp")
)

// Manifest describes metadata shipped with deployment packages.
type Manifest struct {
	RemoteHost  string    `json:"remote_host"`
	XP2PVersion string    `json:"xp2p_version"`
	GeneratedAt time.Time `json:"generated_at"`
}

// Validate ensures the manifest contains required fields.
func Validate(m Manifest) error {
	if strings.TrimSpace(m.RemoteHost) == "" {
		return ErrRemoteHostEmpty
	}
	if strings.TrimSpace(m.XP2PVersion) == "" {
		return ErrVersionEmpty
	}
	if m.GeneratedAt.IsZero() {
		return ErrGeneratedZero
	}
	return nil
}

// Marshal encodes the manifest as formatted JSON.
func Marshal(m Manifest) ([]byte, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	m.GeneratedAt = m.GeneratedAt.UTC()

	type manifest Manifest
	return json.MarshalIndent(manifest(m), "", "  ")
}

// Unmarshal decodes manifest data from JSON.
func Unmarshal(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("xp2p: decode deployment manifest: %w", err)
	}
	if err := Validate(m); err != nil {
		return Manifest{}, err
	}
	m.GeneratedAt = m.GeneratedAt.UTC()
	return m, nil
}

// Read consumes JSON from reader and returns a manifest.
func Read(r io.Reader) (Manifest, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Manifest{}, fmt.Errorf("xp2p: read deployment manifest: %w", err)
	}
	return Unmarshal(data)
}

// Write serialises the manifest to writer using formatted JSON.
func Write(w io.Writer, m Manifest) error {
	data, err := Marshal(m)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("xp2p: write deployment manifest: %w", err)
	}
	return nil
}
