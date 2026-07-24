package root

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func serverHARedirectListContractCase() contractCase {
	args := []string{"server", "ha", "redirect", "list"}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup:    setupHARedirectListCase,
		assertResult: func(t *testing.T, result map[string]any) {
			rules, ok := result["redirects"].([]any)
			if !ok || len(rules) != 2 {
				t.Fatalf("redirects=%#v", result["redirects"])
			}
			domain, ok := rules[0].(map[string]any)
			if !ok || domain["domain"] != "zulu.example" ||
				domain["outbound_tag"] != "channel-zulu-tag" ||
				domain["access"] != "restricted" {
				t.Fatalf("first HA redirect changed: %#v", rules[0])
			}
			users, ok := domain["users"].([]any)
			if !ok || len(users) != 1 || users[0] != "user Ω" {
				t.Fatalf("HA redirect users changed: %#v", domain["users"])
			}
			cidr, ok := rules[1].(map[string]any)
			if !ok || cidr["cidr"] != "192.0.2.0/24" ||
				cidr["outbound_tag"] != "channel-alpha-tag" ||
				cidr["no_routes"] != true {
				t.Fatalf("second HA redirect changed: %#v", rules[1])
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"secret", "password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("HA redirect list leaked credentials: %#v", result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			rules, ok := result["redirects"].([]any)
			if !ok || rules == nil || len(rules) != 0 {
				t.Fatalf("empty redirects must be []: %#v", result["redirects"])
			}
		},
		emptyResult:      "redirects is a non-nil empty array when HA is not configured",
		credentialPolicy: "HA redirect list omits peer and identity credentials",
		edgeCases:        []string{"boolean", "ordered collection", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{"zulu.example", "channel-zulu-tag", "192.0.2.0/24", "channel-alpha-tag"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human baseline is missing %q: output=%q diagnostics=%q", expected, output, diagnostics)
				}
			}
		},
	}
}

func setupHARedirectListCase(t *testing.T, mode string) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	if mode == "empty" {
		return
	}
	fixture := `[server.ha_generation]
number = 1
`
	if mode == "success" {
		fixture = `[server.ha_generation]
number = 7
redirects = [91, 123, 34, 100, 111, 109, 97, 105, 110, 34, 58, 34, 122, 117, 108, 117, 46, 101, 120, 97, 109, 112, 108, 101, 34, 44, 34, 111, 117, 116, 98, 111, 117, 110, 100, 95, 116, 97, 103, 34, 58, 34, 99, 104, 97, 110, 110, 101, 108, 45, 122, 117, 108, 117, 45, 116, 97, 103, 34, 44, 34, 97, 99, 99, 101, 115, 115, 34, 58, 34, 114, 101, 115, 116, 114, 105, 99, 116, 101, 100, 34, 44, 34, 117, 115, 101, 114, 115, 34, 58, 91, 34, 117, 115, 101, 114, 32, 206, 169, 34, 93, 125, 44, 123, 34, 99, 105, 100, 114, 34, 58, 34, 49, 57, 50, 46, 48, 46, 50, 46, 48, 47, 50, 52, 34, 44, 34, 111, 117, 116, 98, 111, 117, 110, 100, 95, 116, 97, 103, 34, 58, 34, 99, 104, 97, 110, 110, 101, 108, 45, 97, 108, 112, 104, 97, 45, 116, 97, 103, 34, 44, 34, 110, 111, 95, 114, 111, 117, 116, 101, 115, 34, 58, 116, 114, 117, 101, 125, 93]

[server.ha_generation.group]
id = "group-id"
tag = "ha-group"

[server.ha_generation.group.selector]
mode = "automatic"
failure_threshold = 2
`
	}
	writeContractFixture(t, filepath.Join(root, layout.ServerConfigFileName), fixture)
}
