//go:build windows

package server

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestListUsersBuildsLinksFromCertificate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(dir, "logs"))
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, true)

	certPath, keyPath := createTestCertificateFiles(t, dir, "links.example.test")
	pendingDir := mustPendingConfigDir(t, configDir)
	if err := os.MkdirAll(pendingDir, 0o755); err != nil {
		t.Fatalf("mkdir pending: %v", err)
	}
	if err := os.Rename(certPath, filepath.Join(pendingDir, "cert.pem")); err != nil {
		t.Fatalf("rename cert: %v", err)
	}
	if err := os.Rename(keyPath, filepath.Join(pendingDir, "key.pem")); err != nil {
		t.Fatalf("rename key: %v", err)
	}

	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "alpha",
		Password:   "secret",
		Host:       "links.example.test",
	}); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	users, err := ListUsers(context.Background(), ListUsersOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		Pending:    true,
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
	pin, err := certificateFingerprintSHA256(filepath.Join(pendingDir, "cert.pem"))
	if err != nil {
		t.Fatalf("fingerprint cert: %v", err)
	}
	query := url.Values{}
	query.Set("pinnedPeerCertSha256", pin)
	query.Set("security", "tls")
	query.Set("sni", "links.example.test")
	query.Set("verifyPeerCertByName", "links.example.test")
	want := "trojan://secret@links.example.test:58443?" + query.Encode() + "#alpha"
	if users[0].Link != want {
		t.Fatalf("unexpected link: %s", users[0].Link)
	}
}

func TestUserLinkRequiresHostWhenTLSDisabled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(dir, "logs"))
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, false)

	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "beta",
		Password:   "secret",
		Host:       "example.internal",
	}); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	_, err := GetUserLink(context.Background(), UserLinkOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "beta",
		Pending:    true,
	})
	if err == nil {
		t.Fatalf("expected error when host missing for non-TLS configuration")
	}

	link, err := GetUserLink(context.Background(), UserLinkOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "beta",
		Host:       "example.internal",
		Pending:    true,
	})
	if err != nil {
		t.Fatalf("UserLink: %v", err)
	}
	if link.Link != "trojan://secret@example.internal:58443?security=none#beta" {
		t.Fatalf("unexpected link: %s", link.Link)
	}
}

func TestListUsersSelfSignedAddsPinnedPeerCert(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(dir, "logs"))
	configDir := filepath.Join(dir, "config-server")
	prepareTrojanConfig(t, configDir, true)

	certPath, keyPath := createTestCertificateFiles(t, dir, "self.example.test")
	pendingDir := mustPendingConfigDir(t, configDir)
	if err := os.MkdirAll(pendingDir, 0o755); err != nil {
		t.Fatalf("mkdir pending: %v", err)
	}
	if err := os.Rename(certPath, filepath.Join(pendingDir, "cert.pem")); err != nil {
		t.Fatalf("rename cert: %v", err)
	}
	if err := os.Rename(keyPath, filepath.Join(pendingDir, "key.pem")); err != nil {
		t.Fatalf("rename key: %v", err)
	}

	if err := AddUser(context.Background(), AddUserOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		UserID:     "alpha",
		Password:   "secret",
		Host:       "self.example.test",
	}); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	users, err := ListUsers(context.Background(), ListUsersOptions{
		InstallDir: dir,
		ConfigDir:  "config-server",
		Pending:    true,
	})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	pin, err := certificateFingerprintSHA256(filepath.Join(pendingDir, "cert.pem"))
	if err != nil {
		t.Fatalf("fingerprint cert: %v", err)
	}
	query := url.Values{}
	query.Set("pinnedPeerCertSha256", pin)
	query.Set("security", "tls")
	query.Set("sni", "self.example.test")
	query.Set("verifyPeerCertByName", "self.example.test")
	want := "trojan://secret@self.example.test:58443?" + query.Encode() + "#alpha"
	if users[0].Link != want {
		t.Fatalf("unexpected link: %s", users[0].Link)
	}
}
