package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestErrorMarkerRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "apply.error")

	in := ErrorMarker{
		RequestID: "  req-1  ",
		Role:      " CLIENT ",
		Reason:    "line1\r\nline2\nline3\rline4",
	}
	if err := WriteError(path, in, ""); err != nil {
		t.Fatalf("WriteError: %v", err)
	}

	out, exists, err := ReadError(path)
	if err != nil {
		t.Fatalf("ReadError: %v", err)
	}
	if !exists {
		t.Fatalf("expected marker to exist")
	}
	if out.RequestID != "req-1" {
		t.Fatalf("request_id mismatch: %q", out.RequestID)
	}
	if out.Role != "client" {
		t.Fatalf("role mismatch: %q", out.Role)
	}
	if out.Reason != "line1 line2 line3 line4" {
		t.Fatalf("reason mismatch: %q", out.Reason)
	}
	if out.Timestamp.IsZero() {
		t.Fatalf("expected timestamp to be set")
	}
}

func TestReadErrorRepairsLiteralNewlineEscapes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "apply.error")
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339Nano)

	// Some environments may accidentally write trailing "\\n" sequences.
	payload := `{"request_id":"req-1","role":"client","timestamp":"` + ts + `","reason":"ok"}` + "\\n"
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, exists, err := ReadError(path)
	if err != nil {
		t.Fatalf("ReadError: %v", err)
	}
	if !exists {
		t.Fatalf("expected marker to exist")
	}
	if out.RequestID != "req-1" {
		t.Fatalf("request_id mismatch: %q", out.RequestID)
	}
	if out.Role != "client" {
		t.Fatalf("role mismatch: %q", out.Role)
	}
	if strings.TrimSpace(out.Reason) != "ok" {
		t.Fatalf("reason mismatch: %q", out.Reason)
	}
}
