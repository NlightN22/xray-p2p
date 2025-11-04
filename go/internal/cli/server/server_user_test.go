package servercmd

import (
	"context"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerUserCommands(t *testing.T) {
	tests := []struct {
		name         string
		cfg          config.Config
		args         []string
		run          func(context.Context, config.Config, []string) int
		setup        func(*testing.T) func()
		wantSnippets []string
	}{
		{
			name: "user add prints link",
			cfg:  serverCfg(`C:\xp2p`, "config-server", "example.test"),
			args: []string{"--path", `C:\xp2p`, "--config-dir", "config-server", "--id", "alpha", "--password", "secret"},
			run:  runServerUserAdd,
			setup: func(t *testing.T) func() {
				restoreAdd := stubServerUserAdd(func(context.Context, server.AddUserOptions) error { return nil })
				restoreLink := stubServerUserLink(func(context.Context, server.UserLinkOptions) (server.UserLink, error) {
					return server.UserLink{
						UserID:   "alpha",
						Password: "secret",
						Link:     "trojan://secret@example.test:62022?allowInsecure=1&security=tls&sni=example.test#alpha",
					}, nil
				})
				return func() {
					restoreLink()
					restoreAdd()
				}
			},
			wantSnippets: []string{"trojan://secret@example.test:62022"},
		},
		{
			name: "user list prints links",
			cfg:  serverCfg(`C:\xp2p`, "config-server", ""),
			args: []string{"list"},
			run:  runServerUser,
			setup: func(*testing.T) func() {
				return stubServerUserList(func(context.Context, server.ListUsersOptions) ([]server.UserLink, error) {
					return []server.UserLink{
						{UserID: "alpha", Link: "trojan://a"},
						{UserID: "", Link: "trojan://b"},
					}, nil
				})
			},
			wantSnippets: []string{"alpha: trojan://a", "(unnamed): trojan://b"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				t.Cleanup(tt.setup(t))
			}
			output := captureStdout(t, func() {
				if code := tt.run(context.Background(), tt.cfg, tt.args); code != 0 {
					t.Fatalf("exit code: %d", code)
				}
			})
			for _, snippet := range tt.wantSnippets {
				if !strings.Contains(output, snippet) {
					t.Fatalf("expected %q in %q", snippet, output)
				}
			}
		})
	}
}
