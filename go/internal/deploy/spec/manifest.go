package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrHostEmpty      = errors.New("xp2p: deploy manifest requires host")
	ErrVersionInvalid = errors.New("xp2p: deploy manifest requires positive version")
	ErrCredentialPair = errors.New("xp2p: deploy manifest user and password must both be set or both be empty")
)

// Manifest represents the encrypted deploy payload shared between client and server.
// It is also reused for offline deployment manifests.
type Manifest struct {
	Host           string `json:"host"`
	Version        int    `json:"version"`
	InstallDir     string `json:"install_dir,omitempty"`
	TrojanPort     string `json:"trojan_port,omitempty"`
	TrojanUser     string `json:"user,omitempty"`
	TrojanPassword string `json:"password,omitempty"`
	ExpiresAt      int64  `json:"exp,omitempty"`
}

// Validate checks required fields and basic consistency.
func Validate(m Manifest) error {
	if strings.TrimSpace(m.Host) == "" {
		return ErrHostEmpty
	}
	if m.Version <= 0 {
		return ErrVersionInvalid
	}
	userPresent := strings.TrimSpace(m.TrojanUser) != ""
	passwordPresent := strings.TrimSpace(m.TrojanPassword) != ""
	if userPresent != passwordPresent {
		return ErrCredentialPair
	}
	return nil
}

// Normalize trims strings and applies defaults.
func Normalize(m Manifest) Manifest {
	if m.Version == 0 {
		m.Version = 2
	}
	m.Host = strings.TrimSpace(m.Host)
	m.InstallDir = strings.TrimSpace(m.InstallDir)
	m.TrojanPort = strings.TrimSpace(m.TrojanPort)
	m.TrojanUser = strings.TrimSpace(m.TrojanUser)
	m.TrojanPassword = strings.TrimSpace(m.TrojanPassword)
	return m
}

// Marshal encodes the manifest as compact JSON after validation.
func Marshal(m Manifest) ([]byte, error) {
	m = Normalize(m)
	if err := Validate(m); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// Unmarshal decodes manifest data from JSON.
func Unmarshal(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("xp2p: decode deploy manifest: %w", err)
	}
	m = Normalize(m)
	if err := Validate(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Read consumes JSON from reader and returns a manifest.
func Read(r io.Reader) (Manifest, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Manifest{}, fmt.Errorf("xp2p: read deploy manifest: %w", err)
	}
	return Unmarshal(data)
}

// Write serialises the manifest to writer using compact JSON.
func Write(w io.Writer, m Manifest) error {
	data, err := Marshal(m)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("xp2p: write deploy manifest: %w", err)
	}
	return nil
}
