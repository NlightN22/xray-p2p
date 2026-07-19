package apply

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadRequestRepairsLiteralNewlineEscapes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "apply.request")

	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339Nano)
	payload := `{"id":"req-1","timestamp":"` + ts + `","role":"client"}` + "\\n"
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	req, exists, err := ReadRequest(path)
	if err != nil {
		t.Fatalf("ReadRequest: %v", err)
	}
	if !exists {
		t.Fatalf("expected request to exist")
	}
	if req.ID != "req-1" {
		t.Fatalf("id mismatch: %q", req.ID)
	}
	if req.Role != "client" {
		t.Fatalf("role mismatch: %q", req.Role)
	}
}

func TestWriteRequestReplacesSameRoleGeneration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apply.request")
	first := Request{ID: "first", Timestamp: time.Now().UTC(), Role: RoleClient}
	second := Request{ID: "second", Timestamp: time.Now().UTC(), Role: RoleClient}
	if err := WriteRequest(path, first, ""); err != nil {
		t.Fatalf("write first request: %v", err)
	}
	if err := WriteRequest(path, second, ""); err != nil {
		t.Fatalf("write second request: %v", err)
	}
	got, exists, err := ReadRequest(path)
	if err != nil || !exists {
		t.Fatalf("read request: exists=%v err=%v", exists, err)
	}
	if got.ID != second.ID {
		t.Fatalf("request ID = %q, want %q", got.ID, second.ID)
	}
}
