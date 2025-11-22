package installstate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
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

func TestHasValidMarkerScenarios(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, FileName)

	ok, err := HasValidMarker(path, KindClient)
	if err != nil {
		t.Fatalf("missing file should not be fatal: %v", err)
	}
	if ok {
		t.Fatalf("expected HasValidMarker to be false when file is absent")
	}

	if err := Write(path, KindClient); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	ok, err = HasValidMarker(path, KindClient)
	if err != nil || !ok {
		t.Fatalf("expected marker to be detected, ok=%v err=%v", ok, err)
	}

	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if ok, err := HasValidMarker(badPath, KindClient); err == nil || ok {
		t.Fatalf("expected error for corrupt state, ok=%v err=%v", ok, err)
	}
}

func TestRolesReturnsCopyAndNormalizesKinds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, FileName)

	now := time.Unix(0, 0).UTC()
	state := fileState{
		Roles: map[Kind]Marker{
			KindClient: {
				Version:     "v1",
				InstalledAt: now,
			},
			KindServer: {
				Version:     "v2",
				InstalledAt: now,
			},
		},
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	roles, err := Roles(path)
	if err != nil {
		t.Fatalf("roles: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("expected two roles, got %d", len(roles))
	}
	if roles[KindClient].Kind != KindClient || roles[KindServer].Kind != KindServer {
		t.Fatalf("kinds should be normalized, got %+v", roles)
	}

	roles[KindClient] = Marker{}
	rolesAgain, err := Roles(path)
	if err != nil {
		t.Fatalf("roles after mutation: %v", err)
	}
	if rolesAgain[KindClient].Version != "v1" {
		t.Fatalf("roles result should be copy, got %#v", rolesAgain[KindClient])
	}
}

func TestReadErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if _, err := Read(path, KindClient); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode error, got %v", err)
	}

	path = filepath.Join(dir, "legacy.json")
	legacy := Marker{
		Kind:        Kind("unknown"),
		Version:     "legacy",
		InstalledAt: time.Now().UTC(),
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	if _, err := Read(path, KindClient); err == nil || !strings.Contains(err.Error(), "unknown marker kind") {
		t.Fatalf("expected unknown kind error, got %v", err)
	}
}

func TestFileNameForKind(t *testing.T) {
	t.Parallel()

	if got := FileNameForKind(KindClient); got != layout.ClientStateFileName {
		t.Fatalf("unexpected client marker name %s", got)
	}
	if got := FileNameForKind(KindServer); got != layout.ServerStateFileName {
		t.Fatalf("unexpected server marker name %s", got)
	}
	if got := FileNameForKind("other"); got != FileName {
		t.Fatalf("unexpected default marker name %s", got)
	}
}
