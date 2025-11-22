package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
)

func TestAddForwardUpdatesStateAndInbounds(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, DefaultClientConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	writeEmptyInbounds(t, filepath.Join(configDir, "inbounds.json"))

	result, err := AddForward(ForwardAddOptions{
		InstallDir:    dir,
		ConfigDir:     DefaultClientConfigDir,
		Target:        "192.0.2.10:8080",
		ListenAddress: "127.0.0.1",
		ListenPort:    61234,
		Protocol:      forward.ProtocolTCP,
	})
	if err != nil {
		t.Fatalf("AddForward returned error: %v", err)
	}
	if result.Rule.ListenPort != 61234 {
		t.Fatalf("unexpected listen port %d", result.Rule.ListenPort)
	}
	if result.Routed {
		t.Fatalf("expected Routed=false when no redirect rules")
	}

	statePath := filepath.Join(dir, installstate.FileNameForKind(installstate.KindClient))
	state, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Forwards) != 1 {
		t.Fatalf("expected 1 forward entry, got %d", len(state.Forwards))
	}
	entry := state.Forwards[0]
	if entry.TargetIP != "192.0.2.10" || entry.TargetPort != 8080 {
		t.Fatalf("unexpected target %+v", entry)
	}
	if entry.Protocol != forward.ProtocolTCP {
		t.Fatalf("unexpected protocol %s", entry.Protocol)
	}

	inbounds := readInbounds(t, filepath.Join(configDir, "inbounds.json"))
	items := inbounds["inbounds"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 inbound entry, got %d", len(items))
	}
	obj := items[0].(map[string]any)
	if obj["remark"] != entry.Remark {
		t.Fatalf("expected remark %s, got %v", entry.Remark, obj["remark"])
	}
}

func TestRemoveForwardCleansState(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, DefaultClientConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	writeEmptyInbounds(t, filepath.Join(configDir, "inbounds.json"))

	if _, err := AddForward(ForwardAddOptions{
		InstallDir:    dir,
		ConfigDir:     DefaultClientConfigDir,
		Target:        "192.0.2.20:9000",
		ListenAddress: "127.0.0.1",
		ListenPort:    61235,
		Protocol:      forward.ProtocolTCP,
	}); err != nil {
		t.Fatalf("AddForward returned error: %v", err)
	}

	if _, err := RemoveForward(ForwardRemoveOptions{
		InstallDir: dir,
		ConfigDir:  DefaultClientConfigDir,
		Selector: forward.Selector{
			ListenPort: 61235,
		},
	}); err != nil {
		t.Fatalf("RemoveForward returned error: %v", err)
	}

	statePath := filepath.Join(dir, installstate.FileNameForKind(installstate.KindClient))
	state, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Forwards) != 0 {
		t.Fatalf("expected forwards cleared, got %+v", state.Forwards)
	}

	inbounds := readInbounds(t, filepath.Join(configDir, "inbounds.json"))
	items := inbounds["inbounds"].([]any)
	if len(items) != 0 {
		t.Fatalf("expected inbounds cleared, got %d", len(items))
	}
}

func writeEmptyInbounds(t *testing.T, path string) {
	t.Helper()
	doc := map[string]any{
		"inbounds": []any{},
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal inbounds: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write inbounds: %v", err)
	}
}

func readInbounds(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read inbounds: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse inbounds: %v", err)
	}
	return doc
}

func TestListForwardsReturnsCopyOfState(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, installstate.FileNameForKind(installstate.KindClient))

	state := clientInstallState{
		Forwards: []forward.Rule{
			{ListenAddress: "127.0.0.1", ListenPort: 10001, Tag: "forward-10001"},
			{ListenAddress: "127.0.0.1", ListenPort: 10002, Tag: "forward-10002"},
		},
	}
	if err := state.save(statePath); err != nil {
		t.Fatalf("write state: %v", err)
	}

	rules, err := ListForwards(ForwardListOptions{
		InstallDir: dir,
		ConfigDir:  DefaultClientConfigDir,
	})
	if err != nil {
		t.Fatalf("ListForwards returned error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 forwards, got %d", len(rules))
	}
	if rules[0].ListenPort != 10001 || rules[1].Tag != "forward-10002" {
		t.Fatalf("unexpected rules: %+v", rules)
	}

	// Mutate the returned slice and ensure persisted state remains intact.
	rules[0].ListenPort = 99999
	reloaded, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if reloaded.Forwards[0].ListenPort != 10001 {
		t.Fatalf("state was modified when rules slice changed: %+v", reloaded.Forwards[0])
	}
}
