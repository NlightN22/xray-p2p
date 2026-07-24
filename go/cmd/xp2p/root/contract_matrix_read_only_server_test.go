package root

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func serverReverseListContractCase() contractCase {
	args := []string{"server", "reverse", "list"}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup: func(t *testing.T, mode string) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			if mode == "empty" {
				return
			}
			fixture := `[server]
reverse_channels = "invalid"
`
			if mode == "success" {
				fixture = `[server.reverse_channels.zulu]
domain = "zulu.rev"
host = "zulu.example"
user_id = "user-zulu"
tag = "outbound-zulu"
disabled = true

[server.reverse_channels.alpha]
domain = "alpha Ω.rev"
host = "alpha example"
user_id = "user\u0001"
tag = "outbound-alpha"
`
			}
			path := filepath.Join(root, layout.ServerConfigFileName)
			if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		assertResult: func(t *testing.T, result map[string]any) {
			tunnels, ok := result["reverse_tunnels"].([]any)
			if !ok || len(tunnels) != 2 {
				t.Fatalf("reverse_tunnels=%#v", result["reverse_tunnels"])
			}
			alpha, ok := tunnels[0].(map[string]any)
			if !ok || alpha["domain"] != "alpha Ω.rev" || alpha["host"] != "alpha example" ||
				alpha["portal_present"] != true ||
				alpha["routing_rule_present"] != true || alpha["enabled"] != true {
				t.Fatalf("first reverse tunnel changed: %#v", tunnels[0])
			}
			zulu, ok := tunnels[1].(map[string]any)
			if !ok || zulu["domain"] != "zulu.rev" || zulu["enabled"] != false {
				t.Fatalf("reverse tunnel order or boolean types changed: %#v", tunnels[1])
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("server reverse list leaked incidental credentials: %#v", result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			tunnels, ok := result["reverse_tunnels"].([]any)
			if !ok || tunnels == nil || len(tunnels) != 0 {
				t.Fatalf("empty reverse_tunnels must be []: %#v", result["reverse_tunnels"])
			}
		},
		emptyResult:      "reverse_tunnels is a non-nil empty array when no Desired file exists",
		credentialPolicy: "list omits passwords, tokens, and private keys",
		edgeCases:        []string{"boolean", "deterministic order", "Unicode/control characters", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{
				"DOMAIN", "HOST", "USER", "OUTBOUND TAG", "PORTAL", "ROUTING RULE", "STATE",
				"alpha Ω.rev", "zulu.rev", "present", "disabled",
			} {
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
