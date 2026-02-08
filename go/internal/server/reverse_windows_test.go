//go:build windows

package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const reverseHost = "edge.example"

func TestAddUserCreatesReverseArtifacts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, true, false)
	writeEmptyRouting(t, filepath.Join(configDir, "routing.json"))

	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "alpha.user",
		Password:   "secret",
		Host:       reverseHost,
	}); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	routingDoc := loadJSONFile(t, filepath.Join(configDir, "routing.json"))
	reverse, ok := routingDoc["reverse"].(map[string]any)
	if !ok {
		t.Fatalf("expected reverse object in routing.json")
	}
	portals := reverse["portals"].([]any)
	if len(portals) != 1 {
		t.Fatalf("expected 1 portal, got %d", len(portals))
	}
	entry := portals[0].(map[string]any)
	if entry["tag"] != "alpha-useredge-example.rev" || entry["domain"] != "alpha-useredge-example.rev" {
		t.Fatalf("unexpected portal entry: %+v", entry)
	}

	rules := routingDoc["routing"].(map[string]any)["rules"].([]any)
	if len(rules) != 3 {
		t.Fatalf("expected 3 routing rules, got %d", len(rules))
	}
	rule := rules[0].(map[string]any)
	if rule["outboundTag"] != "alpha-useredge-example.rev" {
		t.Fatalf("unexpected outbound tag: %+v", rule)
	}
	domains := rule["domain"].([]any)
	if len(domains) != 1 || domains[0] != "full:alpha-useredge-example.rev" {
		t.Fatalf("unexpected domain match: %+v", rule)
	}
	markerCount := 0
	for _, raw := range rules[1:] {
		markerRule := raw.(map[string]any)
		if markerRule["outboundTag"] == "alpha-useredge-example.rev" {
			markerCount++
		}
	}
	if markerCount != 2 {
		t.Fatalf("expected 2 marker rules, got %d", markerCount)
	}

	stateDoc := readServerStateDoc(t, serverStatePath(""))
	channels := stateDoc["reverse_channels"].(map[string]any)
	if _, ok := channels["alpha-useredge-example.rev"]; !ok {
		t.Fatalf("expected reverse channel entry, got %v", channels)
	}
}

func TestRemoveUserCleansReverseArtifacts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, true, false)
	writeEmptyRouting(t, filepath.Join(configDir, "routing.json"))

	opts := AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "beta.user",
		Password:   "secret",
		Host:       reverseHost,
	}
	if err := AddUser(context.Background(), opts); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	if err := RemoveUser(context.Background(), RemoveUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "beta.user",
		Host:       reverseHost,
	}); err != nil {
		t.Fatalf("RemoveUser: %v", err)
	}

	routingDoc := loadJSONFile(t, filepath.Join(configDir, "routing.json"))
	portals := routingDoc["reverse"].(map[string]any)["portals"].([]any)
	if len(portals) != 0 {
		t.Fatalf("expected portals to be empty, got %d", len(portals))
	}
	rules := routingDoc["routing"].(map[string]any)["rules"].([]any)
	if len(rules) != 0 {
		t.Fatalf("expected rules to be empty, got %d", len(rules))
	}

	stateDoc := readServerStateDoc(t, serverStatePath(""))
	if _, ok := stateDoc["reverse_channels"]; ok {
		t.Fatalf("expected reverse_channels to be removed, got %v", stateDoc)
	}
}

func TestAddUserDetectsReverseConflicts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, true, false)
	writeEmptyRouting(t, filepath.Join(configDir, "routing.json"))

	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "gamma.user",
		Password:   "secret",
		Host:       reverseHost,
	}); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "gamma:user",
		Password:   "secret",
		Host:       reverseHost,
	})
	if err == nil {
		t.Fatalf("expected error due to reverse identifier conflict")
	}
}

func TestAddUserRejectsDuplicateUser(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, true, false)
	writeEmptyRouting(t, filepath.Join(configDir, "routing.json"))

	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "alpha.user",
		Password:   "secret",
		Host:       reverseHost,
	}); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "alpha.user",
		Password:   "secret-2",
		Host:       "other.example",
	})
	if err == nil {
		t.Fatalf("expected duplicate user to be rejected")
	}
}

func TestAddUserAllowsForceUpdate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, true, false)
	writeEmptyRouting(t, filepath.Join(configDir, "routing.json"))

	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "alpha.user",
		Password:   "secret",
		Host:       reverseHost,
	}); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "alpha.user",
		Password:   "secret-2",
		Host:       reverseHost,
		Force:      true,
	}); err != nil {
		t.Fatalf("AddUser force: %v", err)
	}

	state, err := loadTrojanState(configDir)
	if err != nil {
		t.Fatalf("loadTrojanState: %v", err)
	}
	if len(state.clients) != 1 {
		t.Fatalf("expected 1 trojan user, got %d", len(state.clients))
	}
	if state.clients[0].Email != "alpha.user" || state.clients[0].Password != "secret-2" {
		t.Fatalf("unexpected user state: %+v", state.clients[0])
	}
}

func writeEmptyRouting(t *testing.T, path string) {
	t.Helper()
	data := []byte(`{"reverse":{"portals":[]},"routing":{"domainStrategy":"AsIs","rules":[]}}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write routing template: %v", err)
	}
}

func loadJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}
