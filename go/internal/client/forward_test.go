package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/testutil"
)

func TestAddForwardUpdatesStateAndInbounds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	extensionsDir := filepath.Join(dir, layout.ClientConfigDir)
	if err := os.MkdirAll(extensionsDir, 0o755); err != nil {
		t.Fatalf("mkdir extensions dir: %v", err)
	}

	reserved := map[int]struct{}{}
	listenPort := findAvailablePort(t, reserved)

	result, err := AddForward(ForwardAddOptions{
		InstallDir:    dir,
		ConfigDir:     DefaultClientConfigDir,
		Target:        "192.0.2.10:8080",
		ListenAddress: "127.0.0.1",
		ListenPort:    listenPort,
		Protocol:      forward.ProtocolTCP,
	})
	if err != nil {
		t.Fatalf("AddForward returned error: %v", err)
	}
	if result.Rule.ListenPort != listenPort {
		t.Fatalf("unexpected listen port %d", result.Rule.ListenPort)
	}
	if result.Routed {
		t.Fatalf("expected Routed=false when no redirect rules")
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	state, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Forwards) != 1 {
		t.Fatalf("expected 1 forward entry, got %d", len(state.Forwards))
	}
	entry := state.Forwards[0]
	if entry.TargetHost != "192.0.2.10" || entry.TargetPort != 8080 {
		t.Fatalf("unexpected target %+v", entry)
	}
	if entry.Protocol != forward.ProtocolTCP {
		t.Fatalf("unexpected protocol %s", entry.Protocol)
	}

	doc := compileDesiredDoc(t, statePath, extensionsDir)
	items, ok := doc["inbounds"].([]any)
	if !ok {
		t.Fatalf("expected inbounds array, got %T", doc["inbounds"])
	}
	if !hasInboundTag(items, entry.Tag) {
		t.Fatalf("expected forward inbound tag %q to be present", entry.Tag)
	}
}

func TestRemoveForwardCleansState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	extensionsDir := filepath.Join(dir, layout.ClientConfigDir)
	if err := os.MkdirAll(extensionsDir, 0o755); err != nil {
		t.Fatalf("mkdir extensions dir: %v", err)
	}

	reserved := map[int]struct{}{}
	listenPort := findAvailablePort(t, reserved)

	if _, err := AddForward(ForwardAddOptions{
		InstallDir:    dir,
		ConfigDir:     DefaultClientConfigDir,
		Target:        "192.0.2.20:9000",
		ListenAddress: "127.0.0.1",
		ListenPort:    listenPort,
		Protocol:      forward.ProtocolTCP,
	}); err != nil {
		t.Fatalf("AddForward returned error: %v", err)
	}

	if _, err := RemoveForward(ForwardRemoveOptions{
		InstallDir: dir,
		ConfigDir:  DefaultClientConfigDir,
		Selector: forward.Selector{
			ListenPort: listenPort,
		},
	}); err != nil {
		t.Fatalf("RemoveForward returned error: %v", err)
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	state, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Forwards) != 0 {
		t.Fatalf("expected forwards cleared, got %+v", state.Forwards)
	}

	doc := compileDesiredDoc(t, statePath, extensionsDir)
	items, ok := doc["inbounds"].([]any)
	if !ok {
		t.Fatalf("expected inbounds array, got %T", doc["inbounds"])
	}
	if hasInboundTag(items, forward.TagForPort(listenPort)) {
		t.Fatalf("expected forward inbound tag %q to be removed", forward.TagForPort(listenPort))
	}
}

func TestRemoveForwardCleanupIgnoresMissingInbound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.MkdirAll(filepath.Join(dir, layout.ClientConfigDir), 0o755); err != nil {
		t.Fatalf("mkdir extensions dir: %v", err)
	}

	reserved := map[int]struct{}{}
	listenPort := findAvailablePort(t, reserved)

	if _, err := AddForward(ForwardAddOptions{
		InstallDir:    dir,
		ConfigDir:     DefaultClientConfigDir,
		Target:        "192.0.2.30:7000",
		ListenAddress: "127.0.0.1",
		ListenPort:    listenPort,
		Protocol:      forward.ProtocolTCP,
	}); err != nil {
		t.Fatalf("AddForward returned error: %v", err)
	}

	if _, err := RemoveForward(ForwardRemoveOptions{
		InstallDir: dir,
		ConfigDir:  DefaultClientConfigDir,
		Selector: forward.Selector{
			ListenPort: listenPort,
		},
		Cleanup: true,
	}); err != nil {
		t.Fatalf("RemoveForward returned error: %v", err)
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	state, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Forwards) != 0 {
		t.Fatalf("expected forwards cleared, got %+v", state.Forwards)
	}
}

func findAvailablePort(t *testing.T, reserved map[int]struct{}) int {
	t.Helper()
	_, port := testutil.FreePort(t)
	if reserved != nil {
		reserved[port] = struct{}{}
	}
	return port
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

func TestListForwardsReturnsCopyOfState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))

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
		Pending:    true,
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
