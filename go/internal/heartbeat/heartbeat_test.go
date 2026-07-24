package heartbeat

import (
	"errors"
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

func TestStoreKeepsMultipleUsersPerTag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ts := time.Date(2025, time.November, 18, 12, 0, 0, 0, time.UTC)
	payloads := []Payload{
		{Tag: "proxy-test", Host: "edge.example.org", User: "alice", ClientIP: "10.0.0.5", Timestamp: ts, RTTMillis: 20},
		{Tag: "proxy-test", Host: "edge.example.org", User: "bob", ClientIP: "10.0.0.6", Timestamp: ts.Add(2 * time.Second), RTTMillis: 25},
	}

	for _, payload := range payloads {
		if _, err := store.Update(payload); err != nil {
			t.Fatalf("Update: %v", err)
		}
	}

	state, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(state.Entries) != 2 {
		t.Fatalf("expected two entries, got %d", len(state.Entries))
	}

	snapshots := state.snapshot(time.Now().UTC(), 10*time.Second)
	if len(snapshots) != 2 {
		t.Fatalf("expected two snapshots, got %d", len(snapshots))
	}
}

func TestExplicitFailureStaysDeadForLongTTL(t *testing.T) {
	store, err := NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	healthy := false
	if _, err := store.Update(Payload{
		Tag: "proxy-test", Host: "edge.example.org", Timestamp: now, Healthy: &healthy,
	}); err != nil {
		t.Fatal(err)
	}
	snapshots := store.Snapshot(now, 70*time.Minute)
	if len(snapshots) != 1 || snapshots[0].Alive {
		t.Fatalf("explicit failure must stay dead: %+v", snapshots)
	}
	if !snapshots[0].Entry.LastSeen.Equal(now) {
		t.Fatalf("failure timestamp was altered: %s != %s", snapshots[0].Entry.LastSeen, now)
	}
}

func TestAutoCapabilityDiscoveryAndRecovery(t *testing.T) {
	store, err := NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 21, 10, 0, 0, 0, time.UTC)
	failed := false
	for attempt := 0; attempt < DiscoveryFailureThreshold; attempt++ {
		entry, updateErr := store.Update(Payload{Tag: "third-party", Host: "edge.example", Timestamp: now.Add(time.Duration(attempt) * time.Second), Healthy: &failed, Mode: ModeAuto, Stage: FailureStageReport})
		if updateErr != nil {
			t.Fatal(updateErr)
		}
		want := StatusProbing
		if attempt+1 == DiscoveryFailureThreshold {
			want = StatusNotDetected
		}
		if entry.Status != want {
			t.Fatalf("attempt %d status = %q, want %q", attempt+1, entry.Status, want)
		}
	}
	healthy := true
	entry, err := store.Update(Payload{Tag: "third-party", Host: "edge.example", Timestamp: now.Add(time.Minute), Healthy: &healthy, Mode: ModeAuto})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Capability != CapabilityXP2PHeartbeat || entry.Status != StatusHealthy {
		t.Fatalf("unexpected detected state: %+v", entry)
	}
	for attempt := 0; attempt < HealthFailureThreshold; attempt++ {
		entry, err = store.Update(Payload{Tag: "third-party", Host: "edge.example", Timestamp: now.Add(2*time.Minute + time.Duration(attempt)*time.Second), Healthy: &failed, Mode: ModeAuto, Stage: FailureStageProbe})
		if err != nil {
			t.Fatal(err)
		}
		want := StatusHealthy
		if attempt+1 == HealthFailureThreshold {
			want = StatusUnhealthy
		}
		if entry.Status != want {
			t.Fatalf("health attempt %d status = %q, want %q", attempt+1, entry.Status, want)
		}
	}
	if entry.Capability != CapabilityXP2PHeartbeat || entry.Status != StatusUnhealthy {
		t.Fatalf("detected capability must persist: %+v", entry)
	}
}

func TestPingOnlyCapabilityPersistsAcrossFailuresAndRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "heartbeat.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC)
	healthy := true
	entry, err := store.Update(Payload{
		Tag: "external", Host: "edge.example", Timestamp: now, Healthy: &healthy,
		Mode: ModeAuto, Capability: CapabilityXP2PDiag,
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Capability != CapabilityXP2PDiag || entry.Status != StatusHealthy {
		t.Fatalf("unexpected ping-only state: %+v", entry)
	}

	failed := false
	for attempt := 0; attempt < HealthFailureThreshold; attempt++ {
		entry, err = store.Update(Payload{
			Tag: "external", Host: "edge.example", Timestamp: now.Add(time.Duration(attempt+1) * time.Second),
			Healthy: &failed, Mode: ModeAuto, Capability: CapabilityXP2PDiag, Stage: FailureStageProbe,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if entry.Capability != CapabilityXP2PDiag || entry.Status != StatusUnhealthy {
		t.Fatalf("ping-only failure was not retained: %+v", entry)
	}

	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	entry, err = reloaded.Update(Payload{
		Tag: "external", Host: "edge.example", Timestamp: now.Add(time.Minute), Healthy: &healthy,
		Mode: ModeAuto, Capability: CapabilityXP2PDiag,
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Status != StatusHealthy || entry.ConsecutiveFailures != 0 || entry.Capability != CapabilityXP2PDiag {
		t.Fatalf("ping-only recovery failed: %+v", entry)
	}
}

func TestFullHeartbeatCapabilityDoesNotDowngrade(t *testing.T) {
	store, _ := NewStore("")
	healthy := true
	entry, err := store.Update(Payload{
		Tag: "server", Host: "server.example", Healthy: &healthy, Mode: ModeAuto,
		Capability: CapabilityXP2PHeartbeat,
	})
	if err != nil {
		t.Fatal(err)
	}
	entry, err = store.Update(Payload{
		Tag: "server", Host: "server.example", Healthy: &healthy, Mode: ModeAuto,
		Capability: CapabilityXP2PDiag,
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Capability != CapabilityXP2PHeartbeat {
		t.Fatalf("full heartbeat capability downgraded: %+v", entry)
	}
}

func TestLegacyDetectedCapabilityLoadsAsFullHeartbeat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "heartbeat.json")
	if err := os.WriteFile(path, []byte(`{"entries":{"edge":{"tag":"edge","host":"edge.example","capability":"detected","status":"healthy"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if capability := store.Capability(""); capability != CapabilityUnknown {
		t.Fatalf("empty endpoint lookup = %q", capability)
	}
	snapshot := store.Snapshot(time.Now(), time.Minute)
	if len(snapshot) != 1 || normalizeCapability(snapshot[0].Entry.Capability) != CapabilityXP2PHeartbeat {
		t.Fatalf("legacy capability is not compatible: %+v", snapshot)
	}
}

func TestSnapshotRejectsExcessiveFutureTimestamp(t *testing.T) {
	now := time.Date(2026, time.July, 21, 10, 0, 0, 0, time.UTC)
	healthy := true
	state := State{Entries: map[string]Entry{"future": {Tag: "future", Host: "edge.example", LastSeen: now.Add(MaxFutureClockSkew + time.Second), Healthy: &healthy}}}
	snapshot := state.Snapshot(now, time.Minute)[0]
	if snapshot.Alive || snapshot.Reason != "clock_skew" {
		t.Fatalf("unexpected future timestamp state: %+v", snapshot)
	}
	state.Entries["future"] = Entry{Tag: "future", Host: "edge.example", LastSeen: now.Add(MaxFutureClockSkew), Healthy: &healthy}
	if snapshot = state.Snapshot(now, time.Minute)[0]; !snapshot.Alive {
		t.Fatalf("timestamp inside skew allowance must be alive: %+v", snapshot)
	}
}

func TestEndpointHostChangeDoesNotInheritCapability(t *testing.T) {
	store, _ := NewStore("")
	healthy := true
	if _, err := store.Update(Payload{Tag: "same", Host: "first.example", User: "alice", Healthy: &healthy, Mode: ModeAuto}); err != nil {
		t.Fatal(err)
	}
	failed := false
	entry, err := store.Update(Payload{Tag: "same", Host: "second.example", User: "alice", Healthy: &failed, Mode: ModeAuto})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Capability != CapabilityUnknown || entry.Status != StatusProbing {
		t.Fatalf("new host inherited prior capability: %+v", entry)
	}
}

func TestLoadXP2P027FixtureUsesLegacyTTLSemantics(t *testing.T) {
	state, err := Load(filepath.Join("testdata", "xp2p-0.2.7.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.January, 1, 0, 0, 30, 0, time.UTC)
	snapshot := state.Snapshot(now, time.Minute)
	if len(snapshot) != 1 || !snapshot[0].Alive {
		t.Fatalf("legacy state is not readable: %+v", snapshot)
	}
}

func TestUpdateRollsBackWhenPersistenceFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	store.persist = func(string, State) error { return errors.New("disk full") }
	healthy := true
	entry, err := store.Update(Payload{Tag: "edge", Host: "edge.example", Healthy: &healthy, Mode: ModeRequired, RTTMillis: 20, RTTValid: true})
	if err == nil {
		t.Fatal("expected persistence failure")
	}
	if entry.FailureStage != FailureStagePersistence {
		t.Fatalf("failure stage = %q", entry.FailureStage)
	}
	if snapshots := store.Snapshot(time.Now(), time.Minute); len(snapshots) != 0 {
		t.Fatalf("failed update changed in-memory state: %+v", snapshots)
	}
	diagnostic, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded := diagnostic.Snapshot(time.Now(), time.Minute)
	if len(loaded) != 1 || loaded[0].Entry.FailureStage != FailureStagePersistence {
		t.Fatalf("persistence diagnostic was not saved: %+v", loaded)
	}
}

func TestFailedAttemptDoesNotChangeRTTAggregates(t *testing.T) {
	store, _ := NewStore("")
	healthy := true
	entry, err := store.Update(Payload{Tag: "edge", Host: "edge.example", Healthy: &healthy, RTTMillis: 20, RTTValid: true})
	if err != nil {
		t.Fatal(err)
	}
	failed := false
	entry, err = store.Update(Payload{Tag: "edge", Host: "edge.example", Healthy: &failed, RTTMillis: 0, RTTValid: false, Stage: FailureStageProbe})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Attempts != 2 || entry.Samples != 1 || entry.MinRTTMillis != 20 || entry.AvgRTTMillis() != 20 {
		t.Fatalf("failed attempt changed RTT aggregates: %+v", entry)
	}
}

func TestVersionedEndpointIDDoesNotOverwritePriorEndpoint(t *testing.T) {
	store, _ := NewStore("")
	healthy := true
	for _, id := range []string{"v1:first", "v1:second"} {
		if _, err := store.Update(Payload{Tag: "same", Host: "edge.example", User: "alice", EndpointID: id, Healthy: &healthy, Mode: ModeAuto}); err != nil {
			t.Fatal(err)
		}
	}
	snapshots := store.Snapshot(time.Now(), time.Minute)
	if len(snapshots) != 2 {
		t.Fatalf("versioned identities were overwritten: %+v", snapshots)
	}
}

func TestSnapshotFreshnessUsesAbsoluteTimeAcrossTimezones(t *testing.T) {
	absolute := time.Date(2026, time.July, 21, 3, 0, 0, 0, time.UTC)
	healthy := true
	state := State{Entries: map[string]Entry{
		"edge": {Tag: "edge", Host: "edge.example", LastSeen: absolute.In(time.FixedZone("UTC+7", 7*60*60)), Healthy: &healthy},
	}}

	utc := state.Snapshot(absolute.Add(15*time.Second), time.Minute)[0]
	local := state.Snapshot(absolute.Add(15*time.Second).In(time.FixedZone("UTC-5", -5*60*60)), time.Minute)[0]
	if !utc.Alive || !local.Alive || utc.Age != local.Age || utc.Age != 15*time.Second {
		t.Fatalf("timezone changed freshness: utc=%+v local=%+v", utc, local)
	}
}

func TestSnapshotFreshnessHandlesSystemClockJumps(t *testing.T) {
	observed := time.Date(2026, time.July, 21, 3, 0, 0, 0, time.UTC)
	healthy := true
	state := State{Entries: map[string]Entry{
		"edge": {Tag: "edge", Host: "edge.example", LastSeen: observed, Healthy: &healthy},
	}}

	withinSkew := state.Snapshot(observed.Add(-MaxFutureClockSkew), time.Minute)[0]
	if !withinSkew.Alive || withinSkew.Age != 0 {
		t.Fatalf("allowed backward jump must have zero age: %+v", withinSkew)
	}
	beyondSkew := state.Snapshot(observed.Add(-MaxFutureClockSkew-time.Nanosecond), time.Minute)[0]
	if beyondSkew.Alive || beyondSkew.Reason != "clock_skew" {
		t.Fatalf("excessive backward jump was accepted: %+v", beyondSkew)
	}
	forward := state.Snapshot(observed.Add(2*time.Minute), time.Minute)[0]
	if forward.Alive || forward.Reason != "expired" {
		t.Fatalf("forward jump did not expire observation: %+v", forward)
	}
}
