package servercmd

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerUserCommands(t *testing.T) {
	t.Run("user add prints link", func(t *testing.T) {
		cfg := serverCfg(`C:\xp2p`, "config-server", "example.test")
		restoreAdd := stubServerUserAdd(func(context.Context, server.AddUserOptions) error { return nil })
		defer restoreAdd()
		restoreLink := stubServerUserLink(func(context.Context, server.UserLinkOptions) (server.UserLink, error) {
			return server.UserLink{
				UserID:   "alpha",
				Password: "secret",
				Link:     "trojan://secret@example.test:58443?pinnedPeerCertSha256=deadbeef&security=tls&sni=example.test&verifyPeerCertByName=example.test#alpha",
			}, nil
		})
		defer restoreLink()

		output := captureStdout(t, func() {
			code := runServerUserAdd(context.Background(), cfg, serverUserAddOptions{
				Path:      `C:\xp2p`,
				ConfigDir: "config-server",
				UserID:    "alpha",
				Password:  "secret",
			})
			if code != 0 {
				t.Fatalf("exit code: %d", code)
			}
		})
		if !strings.Contains(output, "trojan://secret@example.test:58443") {
			t.Fatalf("expected link in output, got %q", output)
		}
	})

	t.Run("user add generates password when missing", func(t *testing.T) {
		cfg := serverCfg(`C:\xp2p`, "config-server", "example.test")
		var captured server.AddUserOptions
		restoreAdd := stubServerUserAdd(func(_ context.Context, opts server.AddUserOptions) error {
			captured = opts
			return nil
		})
		defer restoreAdd()
		restoreLink := stubServerUserLink(func(context.Context, server.UserLinkOptions) (server.UserLink, error) {
			return server.UserLink{
				UserID:   "alpha",
				Password: captured.Password,
				Link:     fmt.Sprintf("trojan://%s@example.test:58443?pinnedPeerCertSha256=deadbeef&security=tls&sni=example.test&verifyPeerCertByName=example.test#alpha", captured.Password),
			}, nil
		})
		defer restoreLink()

		output := captureStdout(t, func() {
			code := runServerUserAdd(context.Background(), cfg, serverUserAddOptions{
				Path:      `C:\xp2p`,
				ConfigDir: "config-server",
				UserID:    "alpha",
			})
			if code != 0 {
				t.Fatalf("exit code: %d", code)
			}
		})
		if strings.TrimSpace(captured.Password) == "" {
			t.Fatalf("expected generated password to be set")
		}
		if !strings.Contains(output, captured.Password) {
			t.Fatalf("expected generated password in output, got %q", output)
		}
	})

	t.Run("user add accepts link", func(t *testing.T) {
		cfg := serverCfg(`C:\xp2p`, "config-server", "")
		var captured server.AddUserOptions
		restoreAdd := stubServerUserAdd(func(_ context.Context, opts server.AddUserOptions) error {
			captured = opts
			return nil
		})
		defer restoreAdd()
		restoreLink := stubServerUserLink(func(context.Context, server.UserLinkOptions) (server.UserLink, error) {
			return server.UserLink{
				UserID: "alpha@example.com",
				Link:   "trojan://secret@example.test:58443?security=tls&sni=example.test#alpha%40example.com",
			}, nil
		})
		defer restoreLink()

		output := captureStdout(t, func() {
			code := runServerUserAdd(context.Background(), cfg, serverUserAddOptions{
				Path:      `C:\xp2p`,
				ConfigDir: "config-server",
				Link:      "trojan://secret@example.test:58443?security=tls&sni=example.test#alpha%40example.com",
			})
			if code != 0 {
				t.Fatalf("exit code: %d", code)
			}
		})
		if captured.UserID != "alpha@example.com" || captured.Password != "secret" || captured.Host != "example.test" {
			t.Fatalf("unexpected add options: %+v", captured)
		}
		if !strings.Contains(output, "trojan://secret@example.test:58443") {
			t.Fatalf("expected link in output, got %q", output)
		}
	})

	t.Run("user add rejects invalid password", func(t *testing.T) {
		cfg := serverCfg(`C:\xp2p`, "config-server", "example.test")
		called := false
		restoreAdd := stubServerUserAdd(func(context.Context, server.AddUserOptions) error {
			called = true
			return nil
		})
		defer restoreAdd()

		code := runServerUserAdd(context.Background(), cfg, serverUserAddOptions{
			Path:      `C:\xp2p`,
			ConfigDir: "config-server",
			UserID:    "alpha",
			Password:  "bad+pass",
		})
		if code != 2 {
			t.Fatalf("exit code: %d", code)
		}
		if called {
			t.Fatalf("expected server user add not to be called")
		}
	})

	t.Run("user list prints links", func(t *testing.T) {
		cfg := serverCfg(`C:\xp2p`, "config-server", "")
		restoreList := stubServerUserList(func(context.Context, server.ListUsersOptions) ([]server.UserLink, error) {
			return []server.UserLink{
				{UserID: "alpha", Link: "trojan://a"},
				{UserID: "", Link: "trojan://b"},
			}, nil
		})
		defer restoreList()

		output := captureStdout(t, func() {
			code := runServerUserList(context.Background(), cfg, serverUserListOptions{})
			if code != 0 {
				t.Fatalf("exit code: %d", code)
			}
		})
		for _, snippet := range []string{"alpha: trojan://a", "(unnamed): trojan://b"} {
			if !strings.Contains(output, snippet) {
				t.Fatalf("expected %q in %q", snippet, output)
			}
		}
	})
}
