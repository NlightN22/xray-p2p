//go:build windows

package server

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestListUsersBuildsLinksFromCertificate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(dir, "logs"))
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	if err := os.MkdirAll(defaultTLSDir(), 0o755); err != nil {
		t.Fatalf("mkdir tls dir: %v", err)
	}
	writeCertificateFile(t, defaultCertPath(), defaultKeyPath(), "links.example.test", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

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
	pin, err := certificateFingerprintSHA256(defaultCertPath())
	if err != nil {
		t.Fatalf("fingerprint cert: %v", err)
	}
	query := url.Values{}
	query.Set("security", "tls")
	query.Set("sni", "links.example.test")
	query.Set("type", "tcp")
	query.Set("xp2p_pin_sha256", pin)
	query.Set("xp2p_verify_name", "links.example.test")
	want := "trojan://secret@links.example.test:58443?" + query.Encode() + "#alpha"
	if users[0].Link != want {
		t.Fatalf("unexpected link: %s", users[0].Link)
	}
}

func TestUserLinkRequiresHostWhenTLSDisabled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(dir, "logs"))
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}

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
	if link.Link != "trojan://secret@example.internal:58443?security=none&type=tcp#beta" {
		t.Fatalf("unexpected link: %s", link.Link)
	}
}

func TestListUsersSelfSignedAddsPinnedPeerCert(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(dir, "logs"))
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	if err := os.MkdirAll(defaultTLSDir(), 0o755); err != nil {
		t.Fatalf("mkdir tls dir: %v", err)
	}
	writeCertificateFile(t, defaultCertPath(), defaultKeyPath(), "self.example.test", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

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
	pin, err := certificateFingerprintSHA256(defaultCertPath())
	if err != nil {
		t.Fatalf("fingerprint cert: %v", err)
	}
	query := url.Values{}
	query.Set("security", "tls")
	query.Set("sni", "self.example.test")
	query.Set("type", "tcp")
	query.Set("xp2p_pin_sha256", pin)
	query.Set("xp2p_verify_name", "self.example.test")
	want := "trojan://secret@self.example.test:58443?" + query.Encode() + "#alpha"
	if users[0].Link != want {
		t.Fatalf("unexpected link: %s", users[0].Link)
	}
}
