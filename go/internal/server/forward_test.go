//go:build windows || linux

package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
)

func TestServerAddForwardUpdatesState(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, DefaultServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	writeServerInboundsFile(t, filepath.Join(configDir, "inbounds.json"))

	result, err := AddForward(ForwardAddOptions{
		InstallDir:    dir,
		ConfigDir:     DefaultServerConfigDir,
		Target:        "198.51.100.5:7000",
		ListenAddress: "127.0.0.1",
		BasePort:      52000,
		Protocol:      forward.ProtocolUDP,
	})
	if err != nil {
		t.Fatalf("AddForward returned error: %v", err)
	}
	if result.Rule.ListenPort <= 0 {
		t.Fatalf("expected listen port to be auto-assigned, got %d", result.Rule.ListenPort)
	}
	if result.Routed {
		t.Fatalf("expected Routed=false without redirect rules")
	}

	statePath := filepath.Join(dir, installstate.FileNameForKind(installstate.KindServer))
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	rawRules, ok := doc[serverForwardRulesKey].([]any)
	if !ok || len(rawRules) != 1 {
		t.Fatalf("expected forward state entry, got %v", doc[serverForwardRulesKey])
	}

	inbounds := readServerInboundsDoc(t, filepath.Join(configDir, "inbounds.json"))
	items := inbounds["inbounds"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 inbound entry, got %d", len(items))
	}
}

func TestServerRemoveForwardClearsState(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, DefaultServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	writeServerInboundsFile(t, filepath.Join(configDir, "inbounds.json"))

	addRes, err := AddForward(ForwardAddOptions{
		InstallDir:    dir,
		ConfigDir:     DefaultServerConfigDir,
		Target:        "198.51.100.6:9000",
		ListenAddress: "127.0.0.1",
		BasePort:      53000,
		Protocol:      forward.ProtocolBoth,
	})
	if err != nil {
		t.Fatalf("AddForward returned error: %v", err)
	}

	if _, err := RemoveForward(ForwardRemoveOptions{
		InstallDir: dir,
		ConfigDir:  DefaultServerConfigDir,
		Selector: forward.Selector{
			ListenPort: addRes.Rule.ListenPort,
		},
	}); err != nil {
		t.Fatalf("RemoveForward returned error: %v", err)
	}

	statePath := filepath.Join(dir, installstate.FileNameForKind(installstate.KindServer))
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if _, ok := doc[serverForwardRulesKey]; ok {
		t.Fatalf("expected forward rules to be removed, got %v", doc[serverForwardRulesKey])
	}

	inbounds := readServerInboundsDoc(t, filepath.Join(configDir, "inbounds.json"))
	items := inbounds["inbounds"].([]any)
	if len(items) != 0 {
		t.Fatalf("expected no inbound entries after removal")
	}
}

func writeServerInboundsFile(t *testing.T, path string) {
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

func readServerInboundsDoc(t *testing.T, path string) map[string]any {
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
