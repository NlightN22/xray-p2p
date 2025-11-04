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
	ErrCredentialPair  = errors.New("xp2p: deployment manifest trojan_user and trojan_password must both be set")
)

// Manifest describes metadata shipped with deployment packages.
type Manifest struct {
	RemoteHost     string    `json:"remote_host"`
	XP2PVersion    string    `json:"xp2p_version"`
	GeneratedAt    time.Time `json:"generated_at"`
	InstallDir     string    `json:"install_dir,omitempty"`
	TrojanUser     string    `json:"trojan_user,omitempty"`
	TrojanPassword string    `json:"trojan_password,omitempty"`
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
	userPresent := strings.TrimSpace(m.TrojanUser) != ""
	passwordPresent := strings.TrimSpace(m.TrojanPassword) != ""
	if userPresent != passwordPresent {
		return ErrCredentialPair
	}
	return nil
}

// Marshal encodes the manifest as formatted JSON.
func Marshal(m Manifest) ([]byte, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	m.GeneratedAt = m.GeneratedAt.UTC()
	m.InstallDir = strings.TrimSpace(m.InstallDir)
	m.RemoteHost = strings.TrimSpace(m.RemoteHost)
	m.XP2PVersion = strings.TrimSpace(m.XP2PVersion)
	m.TrojanUser = strings.TrimSpace(m.TrojanUser)
	m.TrojanPassword = strings.TrimSpace(m.TrojanPassword)

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
	m.InstallDir = strings.TrimSpace(m.InstallDir)
	m.RemoteHost = strings.TrimSpace(m.RemoteHost)
	m.XP2PVersion = strings.TrimSpace(m.XP2PVersion)
	m.TrojanUser = strings.TrimSpace(m.TrojanUser)
	m.TrojanPassword = strings.TrimSpace(m.TrojanPassword)
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
