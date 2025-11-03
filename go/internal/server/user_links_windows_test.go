//go:build windows

package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestListUsersBuildsLinksFromCertificate(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, true)

	certPath, keyPath := createTestCertificateFiles(t, dir, "links.example.test")
	if err := os.Rename(certPath, filepath.Join(configDir, "cert.pem")); err != nil {
		t.Fatalf("rename cert: %v", err)
	}
	if err := os.Rename(keyPath, filepath.Join(configDir, "key.pem")); err != nil {
		t.Fatalf("rename key: %v", err)
	}

	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "alpha",
		Password:   "secret",
	}); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	users, err := ListUsers(context.Background(), ListUsersOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
	})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].UserID != "alpha" {
		t.Fatalf("unexpected user id: %s", users[0].UserID)
	}
	if users[0].Password != "secret" {
		t.Fatalf("unexpected password: %s", users[0].Password)
	}
	if want := "trojan://secret@links.example.test:62022?security=tls&sni=links.example.test#alpha"; users[0].Link != want {
		t.Fatalf("unexpected link: %s", users[0].Link)
	}
}

func TestUserLinkRequiresHostWhenTLSDisabled(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, false)

	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "beta",
		Password:   "secret",
	}); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	_, err := GetUserLink(context.Background(), UserLinkOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "beta",
	})
	if err == nil {
		t.Fatalf("expected error when host missing for non-TLS configuration")
	}

	link, err := GetUserLink(context.Background(), UserLinkOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "beta",
		Host:       "example.internal",
	})
	if err != nil {
		t.Fatalf("UserLink: %v", err)
	}
	if link.Link != "trojan://secret@example.internal:62022?security=none#beta" {
		t.Fatalf("unexpected link: %s", link.Link)
	}
}
