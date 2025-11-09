package servercmd

import (
	"context"
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
				Link:     "trojan://secret@example.test:62022?allowInsecure=1&security=tls&sni=example.test#alpha",
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
		if !strings.Contains(output, "trojan://secret@example.test:62022") {
			t.Fatalf("expected link in output, got %q", output)
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
