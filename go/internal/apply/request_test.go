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
