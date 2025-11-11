package installstate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteReadAndRemoveMultipleRoles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, FileName)

	if err := Write(path, KindClient); err != nil {
		t.Fatalf("write client marker: %v", err)
	}
	if err := Write(path, KindServer); err != nil {
		t.Fatalf("write server marker: %v", err)
	}

	clientMarker, err := Read(path, KindClient)
	if err != nil {
		t.Fatalf("read client marker: %v", err)
	}
	if clientMarker.Kind != KindClient {
		t.Fatalf("expected client kind, got %s", clientMarker.Kind)
	}

	serverMarker, err := Read(path, KindServer)
	if err != nil {
		t.Fatalf("read server marker: %v", err)
	}
	if serverMarker.Kind != KindServer {
		t.Fatalf("expected server kind, got %s", serverMarker.Kind)
	}

	if err := Remove(path, KindClient); err != nil {
		t.Fatalf("remove client marker: %v", err)
	}

	if _, err := Read(path, KindClient); !errors.Is(err, ErrRoleNotInstalled) {
		t.Fatalf("expected ErrRoleNotInstalled, got %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected state file to remain after partial removal: %v", err)
	}

	if err := Remove(path, KindServer); err != nil {
		t.Fatalf("remove server marker: %v", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected install-state file to be removed, got err=%v", err)
	}
}

func TestReadLegacyMarker(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, FileName)

	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	legacy := Marker{
		Kind:        KindClient,
		Version:     "legacy",
		InstalledAt: now,
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy marker: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write legacy marker: %v", err)
	}

	marker, err := Read(path, KindClient)
	if err != nil {
		t.Fatalf("read legacy marker: %v", err)
	}
	if marker.Version != "legacy" {
		t.Fatalf("expected legacy version, got %s", marker.Version)
	}
	if !marker.InstalledAt.Equal(now) {
		t.Fatalf("expected installed_at %s, got %s", now, marker.InstalledAt)
	}
}
