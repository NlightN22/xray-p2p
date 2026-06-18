//go:build linux || windows

package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestServerRemoveRedirectByTagCleansOrphanedDuplicateCIDR(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDir := prepareServerRedirectCleanupConfig(t, dir)

	writeServerStateFile(t, dir, map[string]serverReverseChannel{
		"betahost-example.rev": {
			UserID: "beta",
			Host:   "host.example",
			Tag:    "betahost-example.rev",
			Domain: "betahost-example.rev",
		},
	}, []map[string]any{
		{
			"cidr":         "172.16.16.0/23",
			"outbound_tag": "orphaned-host-example.rev",
		},
		{
			"cidr":         "172.16.16.0/23",
			"outbound_tag": "betahost-example.rev",
		},
	})

	if err := RemoveRedirect(RedirectRemoveOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		Tag:        "orphaned-host-example.rev",
	}); err != nil {
		t.Fatalf("RemoveRedirect by tag failed: %v", err)
	}

	records := listPendingServerRedirects(t, dir, configDir)
	if len(records) != 1 {
		t.Fatalf("expected one redirect to remain, got %+v", records)
	}
	if records[0].Tag != "betahost-example.rev" || records[0].Value != "172.16.16.0/23" {
		t.Fatalf("unexpected remaining redirect: %+v", records[0])
	}
}

func TestServerRemoveUserCleansRedirectsForReverseTag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDir := prepareServerRedirectCleanupConfig(t, dir)

	addServerUserForCleanup(t, dir, configDir, "alpha", "secret-alpha", "alpha.example")
	addServerUserForCleanup(t, dir, configDir, "beta", "secret-beta", "beta.example")
	addServerRedirectForCleanup(t, dir, configDir, "172.16.16.0/23", "alpha.example")
	addServerRedirectForCleanup(t, dir, configDir, "172.16.16.0/23", "beta.example")

	if err := RemoveUser(context.Background(), RemoveUserOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		UserID:     "alpha",
	}); err != nil {
		t.Fatalf("RemoveUser alpha failed: %v", err)
	}

	records := listPendingServerRedirects(t, dir, configDir)
	if len(records) != 1 {
		t.Fatalf("expected beta redirect to remain, got %+v", records)
	}
	if records[0].Hostname != "beta.example" || records[0].Value != "172.16.16.0/23" {
		t.Fatalf("unexpected remaining redirect: %+v", records[0])
	}
}

func prepareServerRedirectCleanupConfig(t *testing.T, dir string) string {
	t.Helper()
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	return configDir
}

func addServerUserForCleanup(t *testing.T, dir, configDir, userID, password, host string) {
	t.Helper()
	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		UserID:     userID,
		Password:   password,
		Host:       host,
	}); err != nil {
		t.Fatalf("AddUser %s failed: %v", userID, err)
	}
}

func addServerRedirectForCleanup(t *testing.T, dir, configDir, cidr, host string) {
	t.Helper()
	if err := AddRedirect(RedirectAddOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		CIDR:       cidr,
		Hostname:   host,
	}); err != nil {
		t.Fatalf("AddRedirect %s failed: %v", host, err)
	}
}

func listPendingServerRedirects(t *testing.T, dir, configDir string) []RedirectRecord {
	t.Helper()
	records, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		Pending:    true,
	})
	if err != nil {
		t.Fatalf("ListRedirects failed: %v", err)
	}
	return records
}
