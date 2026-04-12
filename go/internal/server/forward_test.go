//go:build windows || linux

package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
)

func TestServerAddForwardUpdatesState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
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

	statePath := pendingConfigPath()
	doc, err := loadServerStateDoc(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	rawRules, ok := doc[serverForwardRulesKey].([]any)
	if !ok || len(rawRules) != 1 {
		t.Fatalf("expected forward state entry, got %v", doc[serverForwardRulesKey])
	}

	inbounds := readServerInboundsDoc(t, filepath.Join(pendingConfigDir(configDir), "inbounds.json"))
	items := inbounds["inbounds"].([]any)
	if !hasInboundTag(items, result.Rule.Tag) {
		t.Fatalf("expected forward inbound tag %q to be present", result.Rule.Tag)
	}
}

func TestServerRemoveForwardClearsState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
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

	statePath := pendingConfigPath()
	doc, err := loadServerStateDoc(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if _, ok := doc[serverForwardRulesKey]; ok {
		t.Fatalf("expected forward rules to be removed, got %v", doc[serverForwardRulesKey])
	}

	inbounds := readServerInboundsDoc(t, filepath.Join(pendingConfigDir(configDir), "inbounds.json"))
	items := inbounds["inbounds"].([]any)
	if hasInboundTag(items, addRes.Rule.Tag) {
		t.Fatalf("expected forward inbound tag %q to be removed", addRes.Rule.Tag)
	}
}

func writeServerInboundsFile(t *testing.T, path string) {
	t.Helper()
	doc := map[string]any{
		"inbounds": []any{
			map[string]any{
				"protocol": "trojan",
				"port":     58443,
				"settings": map[string]any{
					"clients": []any{},
				},
				"streamSettings": map[string]any{
					"security": "none",
				},
			},
		},
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

func hasInboundTag(items []any, tag string) bool {
	for _, raw := range items {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if entryTag, ok := entry["tag"].(string); ok && entryTag == tag {
			return true
		}
	}
	return false
}
