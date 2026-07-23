//go:build linux

package dnsforward

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeStateCanonicalFormat(t *testing.T) {
	raw := rawState{Entries: map[string]rawStateEntry{
		"example.test": {
			Target:            "10.0.0.1:53",
			Server:            "127.0.0.1#53331",
			ForwardListenPort: 53331,
			ForwardOwner:      forwardOwnerDNSForward,
		},
	}}

	state, report, err := normalizeState(raw)
	if err != nil {
		t.Fatalf("normalizeState returned error: %v", err)
	}
	if state.Entries["example.test"].ForwardOwner != forwardOwnerDNSForward {
		t.Fatalf("forward owner = %q, want %q", state.Entries["example.test"].ForwardOwner, forwardOwnerDNSForward)
	}
	if len(report.DeprecatedFields) != 0 {
		t.Fatalf("deprecated fields = %v, want none", report.DeprecatedFields)
	}
}

func TestNormalizeStateLegacyAutoForward(t *testing.T) {
	setCurrentAppVersion(t, "0.2.7")
	raw := rawState{Entries: map[string]rawStateEntry{
		"example.test": {
			ForwardListenPort: 53331,
			AutoForward:       boolPtr(true),
		},
	}}

	state, report, err := normalizeState(raw)
	if err != nil {
		t.Fatalf("normalizeState returned error: %v", err)
	}
	if state.Entries["example.test"].ForwardOwner != forwardOwnerDNSForward {
		t.Fatalf("forward owner = %q, want %q", state.Entries["example.test"].ForwardOwner, forwardOwnerDNSForward)
	}
	if len(report.DeprecatedFields) != 1 {
		t.Fatalf("deprecated fields = %v, want one auto_forward field", report.DeprecatedFields)
	}
}

func TestNormalizeStateLegacyAutoForwardFalseMeansNoOwnership(t *testing.T) {
	setCurrentAppVersion(t, "0.2.7")
	raw := rawState{Entries: map[string]rawStateEntry{
		"example.test": {
			ForwardListenPort: 53331,
			AutoForward:       boolPtr(false),
		},
	}}

	state, _, err := normalizeState(raw)
	if err != nil {
		t.Fatalf("normalizeState returned error: %v", err)
	}
	if state.Entries["example.test"].ForwardOwner != "" {
		t.Fatalf("forward owner = %q, want empty", state.Entries["example.test"].ForwardOwner)
	}
}

func TestNormalizeStateMixedSameMeaning(t *testing.T) {
	setCurrentAppVersion(t, "0.2.7")
	raw := rawState{Entries: map[string]rawStateEntry{
		"example.test": {
			ForwardListenPort: 53331,
			ForwardOwner:      forwardOwnerDNSForward,
			AutoForward:       boolPtr(true),
		},
	}}

	state, _, err := normalizeState(raw)
	if err != nil {
		t.Fatalf("normalizeState returned error: %v", err)
	}
	if state.Entries["example.test"].ForwardOwner != forwardOwnerDNSForward {
		t.Fatalf("forward owner = %q, want %q", state.Entries["example.test"].ForwardOwner, forwardOwnerDNSForward)
	}
}

func TestNormalizeStateConflictingLegacyAndCanonicalFields(t *testing.T) {
	raw := rawState{Entries: map[string]rawStateEntry{
		"example.test": {
			ForwardListenPort: 53331,
			ForwardOwner:      forwardOwnerDNSForward,
			AutoForward:       boolPtr(false),
		},
	}}

	if _, _, err := normalizeState(raw); err == nil {
		t.Fatal("expected conflicting fields error")
	}
}

func TestNormalizeStateRejectsAutoForwardAfterRemovedVersion(t *testing.T) {
	setCurrentAppVersion(t, "0.2.8")

	raw := rawState{Entries: map[string]rawStateEntry{
		"example.test": {
			ForwardListenPort: 53331,
			AutoForward:       boolPtr(true),
		},
	}}

	if _, _, err := normalizeState(raw); err == nil {
		t.Fatal("expected removed auto_forward error")
	}
}

func setCurrentAppVersion(t *testing.T, value string) {
	t.Helper()
	restore := currentAppVersion
	currentAppVersion = func() string { return value }
	t.Cleanup(func() { currentAppVersion = restore })
}

func TestNormalizeStateDefaultsMissingEntries(t *testing.T) {
	state, _, err := normalizeState(rawState{})
	if err != nil {
		t.Fatalf("normalizeState returned error: %v", err)
	}
	if state.Entries == nil {
		t.Fatal("expected entries map to be initialized")
	}
}

func TestNormalizeStateRejectsInvalidForwardOwner(t *testing.T) {
	raw := rawState{Entries: map[string]rawStateEntry{
		"example.test": {ForwardOwner: "other"},
	}}

	if _, _, err := normalizeState(raw); err == nil {
		t.Fatal("expected invalid forward_owner error")
	}
}

func TestStateSaveWritesCanonicalFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dns-forward-state.json")
	state := state{Entries: map[string]stateEntry{
		"example.test": {
			Target:            "10.0.0.1:53",
			Server:            "127.0.0.1#53331",
			ForwardListenPort: 53331,
			ForwardOwner:      forwardOwnerDNSForward,
		},
	}}

	if err := state.save(path); err != nil {
		t.Fatalf("save returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved state: %v", err)
	}
	if json.Valid(data) == false {
		t.Fatalf("saved state is not valid JSON: %s", string(data))
	}
	if strings.Contains(string(data), "auto_forward") {
		t.Fatalf("saved state contains deprecated auto_forward: %s", string(data))
	}
	if !strings.Contains(string(data), "forward_owner") {
		t.Fatalf("saved state does not contain forward_owner: %s", string(data))
	}
}

func TestLegacyAutoForwardWritesCanonicalAfterStateChangeInDeprecatedVersion(t *testing.T) {
	restore := currentAppVersion
	currentAppVersion = func() string { return "0.2.7" }
	defer func() { currentAppVersion = restore }()

	path := filepath.Join(t.TempDir(), "dns-forward-state.json")
	legacy := []byte(`{
  "entries": {
    "example.test": {
      "target": "10.0.0.1:53",
      "server": "127.0.0.1#53331",
      "forward_listen_port": 53331,
      "auto_forward": true
    }
  }
}`)
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	state, err := loadState(path)
	if err != nil {
		t.Fatalf("loadState returned error: %v", err)
	}
	entry := state.Entries["example.test"]
	entry.RebindDomain = "example.test"
	state.record("example.test", entry)
	if err := state.save(path); err != nil {
		t.Fatalf("save returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved state: %v", err)
	}
	if strings.Contains(string(data), "auto_forward") {
		t.Fatalf("saved state contains deprecated auto_forward: %s", string(data))
	}
	if !strings.Contains(string(data), `"forward_owner": "dns-forward"`) {
		t.Fatalf("saved state does not contain canonical forward_owner: %s", string(data))
	}
}

func boolPtr(value bool) *bool {
	return &value
}
