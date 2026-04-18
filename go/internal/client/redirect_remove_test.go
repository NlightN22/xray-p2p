package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func TestRemoveRedirectByTag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDirName := layout.ClientConfigDir
	if err := os.MkdirAll(filepath.Join(dir, configDirName), 0o755); err != nil {
		t.Fatalf("mkdir extensions dir: %v", err)
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	state := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{
				Hostname: "server.example",
				Tag:      "proxy-server-example",
				Address:  "203.0.113.10",
			},
		},
		Redirects: []redirect.Rule{
			{CIDR: "10.70.0.0/16", OutboundTag: "proxy-server-example"},
			{CIDR: "10.90.0.0/16", OutboundTag: "proxy-server-example"},
		},
	}
	if err := state.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	opts := RedirectRemoveOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
		CIDR:       "10.90.0.0/16",
		Tag:        "proxy-server-example",
	}
	if err := RemoveRedirect(opts); err != nil {
		t.Fatalf("RemoveRedirect failed: %v", err)
	}

	updated, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(updated.Redirects) != 1 {
		t.Fatalf("expected 1 redirect remaining, got %d", len(updated.Redirects))
	}
	if updated.Redirects[0].CIDR != "10.70.0.0/16" {
		t.Fatalf("unexpected remaining CIDR %s", updated.Redirects[0].CIDR)
	}

	list, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
		Pending:    true,
	})
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 list entry, got %d", len(list))
	}
	if list[0].Value != "10.70.0.0/16" || list[0].Tag != "proxy-server-example" {
		t.Fatalf("unexpected list entry %+v", list[0])
	}
}

func TestRemoveDomainRedirect(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDirName := layout.ClientConfigDir
	extensionsDir := filepath.Join(dir, configDirName)
	if err := os.MkdirAll(extensionsDir, 0o755); err != nil {
		t.Fatalf("mkdir extensions dir: %v", err)
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	state := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{
				Hostname: "server.example",
				Tag:      "proxy-server-example",
				Address:  "203.0.113.10",
			},
		},
		Redirects: []redirect.Rule{
			{Domain: "api.example.com", OutboundTag: "proxy-server-example"},
			{CIDR: "10.90.0.0/16", OutboundTag: "proxy-server-example"},
		},
	}
	if err := state.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	opts := RedirectRemoveOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
		Domain:     "api.example.com",
	}
	if err := RemoveRedirect(opts); err != nil {
		t.Fatalf("RemoveRedirect failed: %v", err)
	}

	updated, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(updated.Redirects) != 1 || updated.Redirects[0].CIDR != "10.90.0.0/16" {
		t.Fatalf("unexpected remaining redirects %+v", updated.Redirects)
	}

	doc := compileDesiredDoc(t, statePath, extensionsDir)
	rules := extractRoutingRules(t, doc)
	if hasAnyDomainRedirectRule(rules) {
		t.Fatalf("found domain rule after removal: %+v", rules)
	}
}
