package root

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func clientStateContractCase() contractCase {
	args := []string{"client", "state", "--path", safeServerInstallDir(), "--pending"}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup: func(t *testing.T, mode string) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			fixture := `[client]
`
			if mode == "error" {
				fixture = `[client]
[[client.endpoints]]
hostname = "invalid.example"
tag = "invalid"
heartbeat_mode = "unsupported"
`
			} else if mode == "success" {
				fixture = `[client]
[[client.endpoints]]
hostname = "zulu Ω.example"
tag = "zulu"
address = "192.0.2.2"
port = 443
user = "user-zulu"
heartbeat_mode = "disabled"

[[client.endpoints]]
hostname = "alpha example"
tag = "alpha"
address = "192.0.2.1"
port = 8443
user = "user-alpha\u0001"
heartbeat_mode = "auto"
`
			}
			writeContractFixture(t, filepath.Join(root, layout.ClientConfigFileName), fixture)
		},
		assertResult: func(t *testing.T, result map[string]any) {
			tunnels, ok := result["tunnels"].([]any)
			if !ok || len(tunnels) != 2 {
				t.Fatalf("tunnels=%#v", result["tunnels"])
			}
			alpha, ok := tunnels[0].(map[string]any)
			if !ok || alpha["tag"] != "alpha" || alpha["host"] != "alpha example" ||
				alpha["user"] != "user-alpha\x01" || alpha["alive"] != false || alpha["mode"] != "auto" ||
				alpha["age_millis"] != float64(0) || alpha["samples"] != float64(0) {
				t.Fatalf("first tunnel JSON types changed: %#v", tunnels[0])
			}
			zulu, ok := tunnels[1].(map[string]any)
			if !ok || zulu["tag"] != "zulu" || zulu["host"] != "zulu Ω.example" ||
				zulu["status"] != "disabled" {
				t.Fatalf("tunnel order or status changed: %#v", tunnels[1])
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("client state leaked incidental credentials: %#v", result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			tunnels, ok := result["tunnels"].([]any)
			if !ok || tunnels == nil || len(tunnels) != 0 {
				t.Fatalf("empty tunnels must be []: %#v", result["tunnels"])
			}
		},
		emptyResult:      "tunnels is a non-nil empty array when no endpoints exist",
		credentialPolicy: "state omits endpoint passwords, tokens, and private keys",
		edgeCases:        []string{"number", "boolean", "nullable timestamps", "Unicode/spaces", "ANSI-free streams"},
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
