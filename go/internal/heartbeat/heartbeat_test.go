package heartbeat

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreUpdateAndSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ts := time.Date(2025, time.November, 18, 12, 0, 0, 0, time.UTC)
	payload := Payload{
		Tag:       "proxy-test",
		Host:      "edge.example.org",
		ClientIP:  "10.0.0.5",
		Timestamp: ts,
		RTTMillis: 25,
	}

	if _, err := store.Update(payload); err != nil {
		t.Fatalf("Update: %v", err)
	}
	payload.RTTMillis = 30
	payload.Timestamp = payload.Timestamp.Add(2 * time.Second)
	if _, err := store.Update(payload); err != nil {
		t.Fatalf("Update second: %v", err)
	}

	state, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(state.Entries) != 1 {
		t.Fatalf("expected single entry, got %d", len(state.Entries))
	}

	snapshots := state.snapshot(payload.Timestamp, 10*time.Second)
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(snapshots))
	}

	item := snapshots[0]
	if !item.Alive {
		t.Fatalf("expected alive snapshot")
	}
	if item.AvgRTTMillis != 27.5 {
		t.Fatalf("expected average 27.5, got %v", item.AvgRTTMillis)
	}
	if item.Entry.MinRTTMillis != 25 || item.Entry.MaxRTTMillis != 30 {
		t.Fatalf("unexpected min/max %+v", item.Entry)
	}
}

func TestUpdateRequiresTagAndHost(t *testing.T) {
	store, err := NewStore("")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if _, err := store.Update(Payload{}); err == nil {
		t.Fatalf("expected error for missing tag/host")
	}
	if _, err := store.Update(Payload{Tag: "proxy", Host: ""}); err == nil {
		t.Fatalf("expected error for missing host")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "state.json")
	state := State{}
	if err := Save(path, state); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state not written: %v", err)
	}
}
