//go:build windows || linux

package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestServerAddForwardUpdatesState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	extensionsDir := filepath.Join(dir, layout.ServerConfigDir)

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

	compiled := compileDesiredDoc(t, pendingConfigPath(), extensionsDir)
	items, ok := compiled["inbounds"].([]any)
	if !ok {
		t.Fatalf("expected inbounds array, got %T", compiled["inbounds"])
	}
	if !hasInboundTag(items, result.Rule.Tag) {
		t.Fatalf("expected forward inbound tag %q to be present", result.Rule.Tag)
	}
}

func TestServerRemoveForwardClearsState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	extensionsDir := filepath.Join(dir, layout.ServerConfigDir)

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

	compiled := compileDesiredDoc(t, pendingConfigPath(), extensionsDir)
	items, ok := compiled["inbounds"].([]any)
	if !ok {
		t.Fatalf("expected inbounds array, got %T", compiled["inbounds"])
	}
	if hasInboundTag(items, addRes.Rule.Tag) {
		t.Fatalf("expected forward inbound tag %q to be removed", addRes.Rule.Tag)
	}
}

func TestServerRemoveMissingForwardDoesNotWriteApplyRequest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	doc := map[string]any{
		serverForwardRulesKey: []forward.Rule{
			{ListenAddress: "127.0.0.1", ListenPort: 53001, Tag: "forward-53001"},
		},
	}
	if err := writeServerStateDoc(pendingConfigPath(), doc); err != nil {
		t.Fatalf("write state: %v", err)
	}

	_, err := RemoveForward(ForwardRemoveOptions{
		InstallDir: dir,
		ConfigDir:  DefaultServerConfigDir,
		Selector: forward.Selector{
			ListenPort: 53002,
		},
	})
	if err == nil {
		t.Fatalf("expected missing forward error")
	}
	if _, statErr := os.Stat(config.ApplyRequestPath()); !os.IsNotExist(statErr) {
		t.Fatalf("apply request should not be written for missing forward: %v", statErr)
	}
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
