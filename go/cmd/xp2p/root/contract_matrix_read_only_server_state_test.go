package root

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func serverStateContractCase() contractCase {
	args := []string{"server", "state", "--path", safeServerInstallDir()}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup: func(t *testing.T, mode string) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			fixture := "[server]\n"
			if mode == "error" {
				fixture = "[server\n"
			} else if mode == "success" {
				fixture = `[server]
trojan_users = [
  {email = "user-zulu", password = "matrix-secret-zulu"},
  {email = "user-alpha", password = "matrix-secret-alpha"}
]

[server.reverse_channels.zulu]
user_id = "user-zulu"
host = "zulu Ω.example"

[server.reverse_channels.alpha]
user_id = "user-alpha"
host = "alpha example"
`
			}
			writeContractFixture(t, filepath.Join(root, layout.ServerConfigFileName), fixture)
		},
		assertResult: func(t *testing.T, result map[string]any) {
			tunnels, ok := result["tunnels"].([]any)
			if !ok || len(tunnels) != 2 {
				t.Fatalf("tunnels=%#v", result["tunnels"])
			}
			alpha, ok := tunnels[0].(map[string]any)
			if !ok || alpha["tag"] != "alpha" || alpha["host"] != "alpha example" ||
				alpha["user"] != "user-alpha" || alpha["alive"] != false ||
				alpha["age_millis"] != float64(0) || alpha["samples"] != float64(0) {
				t.Fatalf("first server tunnel changed: %#v", tunnels[0])
			}
			zulu, ok := tunnels[1].(map[string]any)
			if !ok || zulu["tag"] != "zulu" || zulu["host"] != "zulu Ω.example" ||
				zulu["last_seen"] != nil || zulu["healthy"] != nil {
				t.Fatalf("server tunnel order or nullable fields changed: %#v", tunnels[1])
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"matrix-secret", "password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("server state leaked incidental credentials: %#v", result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			tunnels, ok := result["tunnels"].([]any)
			if !ok || tunnels == nil || len(tunnels) != 0 {
				t.Fatalf("empty tunnels must be []: %#v", result["tunnels"])
			}
		},
		emptyResult:      "tunnels is a non-nil empty array when no users or reverse channels exist",
		credentialPolicy: "state omits user passwords, tokens, and private keys",
		edgeCases:        []string{"number", "boolean", "nullable timestamps", "ordered collection", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{"TAG", "HOST", "STATUS", "LAST_RTT", "CLIENT_USER", "zulu Ω.example", "alpha example"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human baseline is missing %q: output=%q diagnostics=%q", expected, output, diagnostics)
				}
			}
			if !strings.Contains(diagnostics, "INFO xp2p starting") {
				t.Fatalf("human diagnostic baseline changed: %q", diagnostics)
			}
		},
	}
}
