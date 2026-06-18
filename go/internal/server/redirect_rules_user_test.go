//go:build linux || windows

package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestServerAddRedirectResolvesReverseUser(t *testing.T) {
	dir := prepareServerRedirectUserTest(t)
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	writeServerStateFile(t, dir, map[string]serverReverseChannel{
		"ab-pushkinain1-example.rev": {
			UserID: "AB-Pushkina",
			Host:   "in1.example",
			Tag:    "ab-pushkinain1-example.rev",
			Domain: "ab-pushkinain1-example.rev",
		},
	}, nil)

	if err := AddRedirect(RedirectAddOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		CIDR:       "192.168.105.1/24",
		User:       "AB-Pushkina",
	}); err != nil {
		t.Fatalf("AddRedirect failed: %v", err)
	}

	stateDoc := readServerStateDoc(t, pendingConfigPath())
	rawRules, ok := stateDoc[serverRedirectRulesKey].([]any)
	if !ok || len(rawRules) != 1 {
		t.Fatalf("expected redirect entry, got %+v", stateDoc[serverRedirectRulesKey])
	}
	rule, ok := rawRules[0].(map[string]any)
	if !ok {
		t.Fatalf("expected redirect map, got %+v", rawRules[0])
	}
	if rule["outbound_tag"] != "ab-pushkinain1-example.rev" {
		t.Fatalf("expected resolved reverse tag, got %+v", rule)
	}
}

func TestServerAddRedirectKeepsTagSeparateFromReverseUser(t *testing.T) {
	dir := prepareServerRedirectUserTest(t)
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	writeServerStateFile(t, dir, map[string]serverReverseChannel{
		"ab-pushkinain1-example.rev": {
			UserID: "AB-Pushkina",
			Host:   "in1.example",
			Tag:    "ab-pushkinain1-example.rev",
			Domain: "ab-pushkinain1-example.rev",
		},
	}, nil)

	err := AddRedirect(RedirectAddOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		CIDR:       "192.168.105.1/24",
		Tag:        "AB-Pushkina",
	})
	if err == nil || !strings.Contains(err.Error(), "outbound tag") {
		t.Fatalf("expected outbound tag error, got %v", err)
	}
}

func TestServerAddRedirectRejectsAmbiguousReverseUser(t *testing.T) {
	dir := prepareServerRedirectUserTest(t)
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	writeServerStateFile(t, dir, map[string]serverReverseChannel{
		"branch-a.rev": {
			UserID: "branch",
			Host:   "a.example",
			Tag:    "branch-a.rev",
			Domain: "branch-a.rev",
		},
		"branch-b.rev": {
			UserID: "branch",
			Host:   "b.example",
			Tag:    "branch-b.rev",
			Domain: "branch-b.rev",
		},
	}, nil)

	err := AddRedirect(RedirectAddOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		CIDR:       "192.168.105.1/24",
		User:       "branch",
	})
	if err == nil || !strings.Contains(err.Error(), "matches multiple portals") {
		t.Fatalf("expected ambiguous reverse user error, got %v", err)
	}
}

func prepareServerRedirectUserTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	return dir
}
