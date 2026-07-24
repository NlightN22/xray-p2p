package root

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func serverHAChannelInspectContractCase() contractCase {
	return contractCase{
		coverage: contractCovered,
		success:  []string{"server", "ha", "channel", "inspect", "channel-zulu"},
		empty:    []string{"server", "ha", "channel", "inspect", "channel-empty"},
		failure:  []string{"server", "ha", "channel", "inspect", "channel-missing"},
		setup:    setupHAChannelInspectCase,
		assertResult: func(t *testing.T, result map[string]any) {
			if result["id"] != "channel-zulu" || result["tag"] != "channel-zulu-tag" ||
				result["domain"] != "zulu Ω.rev" || result["user_id"] != "user-zulu\x01" {
				t.Fatalf("HA channel inspect changed: %#v", result)
			}
			binding, ok := result["binding"].(map[string]any)
			if !ok || binding["group_tag"] != "ha-group" {
				t.Fatalf("HA channel binding changed: %#v", result["binding"])
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"secret", "password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("HA channel inspect leaked credentials: %#v", result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			if result["id"] != "channel-empty" || result["tag"] != "empty-tag" ||
				result["domain"] != "empty.example" {
				t.Fatalf("minimal HA channel changed: %#v", result)
			}
			if _, exists := result["user_id"]; exists {
				t.Fatalf("empty optional user_id must be omitted: %#v", result)
			}
			binding, ok := result["binding"].(map[string]any)
			if !ok || binding["disabled"] != true {
				t.Fatalf("minimal channel must retain disabled=true: %#v", result["binding"])
			}
			for _, key := range []string{"group_tag", "endpoint_tag"} {
				if _, exists := binding[key]; exists {
					t.Fatalf("empty optional binding %s must be omitted: %#v", key, binding)
				}
			}
		},
		emptyResult:      "a minimal disabled channel omits empty optional identity and binding fields",
		credentialPolicy: "channel inspect omits peer secrets and identity credentials",
		edgeCases:        []string{"boolean", "omitted optionals", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            []string{"server", "ha", "channel", "inspect", "channel-zulu"},
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{"ID: channel-zulu", "Tag: channel-zulu-tag", "Domain: zulu Ω.rev", "Group: ha-group", "Disabled: false"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human baseline is missing %q: output=%q diagnostics=%q", expected, output, diagnostics)
				}
			}
		},
	}
}

func setupHAChannelInspectCase(t *testing.T, mode string) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	channel := `{id = "channel-zulu", tag = "channel-zulu-tag", domain = "zulu Ω.rev", user_id = "user-zulu\u0001", binding = {group_tag = "ha-group"}}`
	if mode == "empty" {
		channel = `{id = "channel-empty", tag = "empty-tag", domain = "empty.example", binding = {disabled = true}}`
	}
	fixture := fmt.Sprintf(`[server.ha_generation]
number = 7
channels = [%s]

[server.ha_generation.group]
id = "group-id"
tag = "ha-group"

[server.ha_generation.group.selector]
mode = "automatic"
failure_threshold = 2
`, channel)
	writeContractFixture(t, filepath.Join(root, layout.ServerConfigFileName), fixture)
}
