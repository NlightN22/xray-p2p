package root

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func serverHAPeerListContractCase() contractCase {
	args := []string{"server", "ha", "peer", "list"}
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
ha_peers = "invalid"
`
			if mode == "success" {
				fixture = `[server]
[[server.ha_peers]]
id = "zulu Ω\u0001"
endpoint = "https://zulu.example:8443"
allow_insecure = true
witness = true
non_voting = false
secret = "matrix-secret-zulu"

[[server.ha_peers]]
id = "alpha"
endpoint = "https://alpha.example:8443"
allow_insecure = false
witness = false
non_voting = true
secret = "matrix-secret-alpha"
`
			}
			writeContractFixture(t, filepath.Join(root, layout.ServerConfigFileName), fixture)
		},
		assertResult: func(t *testing.T, result map[string]any) {
			peers, ok := result["peers"].([]any)
			if !ok || len(peers) != 2 {
				t.Fatalf("peers=%#v", result["peers"])
			}
			zulu, ok := peers[0].(map[string]any)
			if !ok || zulu["id"] != "zulu Ω\x01" || zulu["endpoint"] != "https://zulu.example:8443" ||
				zulu["allow_insecure"] != true || zulu["witness"] != true || zulu["non_voting"] != false {
				t.Fatalf("first peer changed: %#v", peers[0])
			}
			alpha, ok := peers[1].(map[string]any)
			if !ok || alpha["id"] != "alpha" || alpha["non_voting"] != true {
				t.Fatalf("peer order or booleans changed: %#v", peers[1])
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"secret", "matrix-secret", "password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("HA peer list leaked credentials: %#v", result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			peers, ok := result["peers"].([]any)
			if !ok || peers == nil || len(peers) != 0 {
				t.Fatalf("empty peers must be []: %#v", result["peers"])
			}
		},
		emptyResult:      "peers is a non-nil empty array when no peers exist",
		credentialPolicy: "peer list omits replication secrets",
		edgeCases:        []string{"boolean", "ordered collection", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{"zulu Ω", "https://zulu.example:8443", "alpha", "https://alpha.example:8443"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human baseline is missing %q: output=%q diagnostics=%q", expected, output, diagnostics)
				}
			}
			if strings.Contains(output, "matrix-secret") {
				t.Fatalf("human peer list leaked secrets: %q", output)
			}
			if !strings.Contains(diagnostics, "INFO xp2p starting") {
				t.Fatalf("human diagnostic baseline changed: %q", diagnostics)
			}
		},
	}
}

func serverHACollectionContractCase(kind string) contractCase {
	args := []string{"server", "ha", kind, "list"}
	resultKey := kind + "s"
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup:    setupHAGenerationCase,
		assertResult: func(t *testing.T, result map[string]any) {
			items, ok := result[resultKey].([]any)
			if !ok || len(items) != 2 {
				t.Fatalf("%s=%#v", resultKey, result[resultKey])
			}
			if kind == "member" {
				assertHAMembers(t, items)
			} else {
				assertHAChannels(t, items)
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"secret", "password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("HA %s list leaked credentials: %#v", kind, result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			items, ok := result[resultKey].([]any)
			if !ok || items == nil || len(items) != 0 {
				t.Fatalf("empty %s must be []: %#v", resultKey, result[resultKey])
			}
		},
		emptyResult:      resultKey + " is a non-nil empty array when HA is not configured",
		credentialPolicy: "HA topology lists omit replication and identity credentials",
		edgeCases:        []string{"number", "boolean", "ordered collection", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			expected := []string{"member-zulu", "member-alpha", "zulu-tag", "alpha-tag"}
			if kind == "channel" {
				expected = []string{"channel-zulu", "channel-alpha", "zulu Ω.rev", "alpha.rev", "disabled"}
			}
			for _, value := range expected {
				if !strings.Contains(output, value) {
					t.Fatalf("human baseline is missing %q: output=%q diagnostics=%q", value, output, diagnostics)
				}
			}
			if !strings.Contains(diagnostics, "INFO xp2p starting") {
				t.Fatalf("human diagnostic baseline changed: %q", diagnostics)
			}
		},
	}
}

func setupHAGenerationCase(t *testing.T, mode string) {
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
		fixture = `[server]
ha_local_peer_id = "peer-alpha\u0001"

[[server.ha_peers]]
id = "peer-zulu"
endpoint = "https://zulu.example:8443"
secret = "matrix-secret-zulu"

[[server.ha_peers]]
id = "peer-alpha\u0001"
endpoint = "https://alpha.example:8443"
secret = "matrix-secret-alpha"

[server.ha_generation]
number = 7
channels = [
  {id = "channel-zulu", tag = "channel-zulu-tag", domain = "zulu Ω.rev", user_id = "user-zulu\u0001", binding = {group_tag = "ha-group"}},
  {id = "channel-alpha", tag = "channel-alpha-tag", domain = "alpha.rev", user_id = "user-alpha", binding = {disabled = true}}
]

[server.ha_generation.group]
id = "group-id\u0001"
tag = "ha-group"
members = [
  {id = "member-zulu", tag = "zulu-tag\u0001", host = "zulu.example", port = 443, profile = "trojan-tls", priority = 20, confirmed = true},
  {id = "member-alpha", tag = "alpha-tag", host = "alpha.example", port = 8443, profile = "trojan-tls", priority = 10, confirmed = true}
]

[server.ha_generation.group.selector]
mode = "automatic"
failure_threshold = 2
success_threshold = 1
cooldown_seconds = 30
minimum_hold_seconds = 15
automatic_failback = true
`
	}
	writeContractFixture(t, filepath.Join(root, layout.ServerConfigFileName), fixture)
}

func assertHAMembers(t *testing.T, items []any) {
	t.Helper()
	zulu, ok := items[0].(map[string]any)
	if !ok || zulu["id"] != "member-zulu" || zulu["tag"] != "zulu-tag\x01" || zulu["port"] != float64(443) ||
		zulu["priority"] != float64(20) || zulu["confirmed"] != true {
		t.Fatalf("first HA member changed: %#v", items[0])
	}
	alpha, ok := items[1].(map[string]any)
	if !ok || alpha["id"] != "member-alpha" || alpha["port"] != float64(8443) {
		t.Fatalf("HA member order or types changed: %#v", items[1])
	}
}

func assertHAChannels(t *testing.T, items []any) {
	t.Helper()
	zulu, ok := items[0].(map[string]any)
	if !ok || zulu["id"] != "channel-zulu" || zulu["domain"] != "zulu Ω.rev" || zulu["user_id"] != "user-zulu\x01" {
		t.Fatalf("first HA channel changed: %#v", items[0])
	}
	alpha, ok := items[1].(map[string]any)
	if !ok || alpha["id"] != "channel-alpha" {
		t.Fatalf("HA channel order changed: %#v", items[1])
	}
	binding, ok := alpha["binding"].(map[string]any)
	if !ok || binding["disabled"] != true {
		t.Fatalf("HA channel binding boolean changed: %#v", alpha["binding"])
	}
}

func serverHAStatusContractCase() contractCase {
	args := []string{"server", "ha", "status"}
	return contractCase{
		coverage: contractCovered, success: args, empty: args, failure: args,
		setup: setupHAGenerationCase,
		assertResult: func(t *testing.T, result map[string]any) {
			if result["configured"] != true || result["generation"] != float64(7) ||
				result["group"] != "ha-group" || result["member_count"] != float64(2) ||
				result["channel_count"] != float64(2) {
				t.Fatalf("HA status changed: %#v", result)
			}
			voters, ok := result["voting_membership"].([]any)
			if !ok || voters == nil || len(voters) == 0 {
				t.Fatalf("voting_membership must be a non-empty array: %#v", result["voting_membership"])
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			if result["configured"] != false {
				t.Fatalf("empty HA status must be unconfigured: %#v", result)
			}
		},
		emptyResult:      "configured is false when no HA generation exists",
		credentialPolicy: "status omits peer secrets and identity credentials",
		edgeCases:        []string{"number", "boolean", "array", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{"Generation: 7", "Group: ha-group", "Members: 2", "Channels: 2", "Quorum:"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human baseline is missing %q: output=%q diagnostics=%q", expected, output, diagnostics)
				}
			}
		},
	}
}

func serverHAGroupInspectContractCase() contractCase {
	args := []string{"server", "ha", "group", "inspect"}
	return contractCase{
		coverage: contractCovered, success: args, empty: args, failure: args,
		setup: setupHAGenerationCase,
		assertResult: func(t *testing.T, result map[string]any) {
			if result["id"] != "group-id\x01" || result["tag"] != "ha-group" {
				t.Fatalf("HA group changed: %#v", result)
			}
			members, ok := result["members"].([]any)
			if !ok || len(members) != 2 {
				t.Fatalf("HA group members changed: %#v", result["members"])
			}
			selector, ok := result["selector"].(map[string]any)
			if !ok || selector["failure_threshold"] != float64(2) ||
				selector["automatic_failback"] != true {
				t.Fatalf("HA selector types changed: %#v", result["selector"])
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			members, ok := result["members"].([]any)
			if !ok || members == nil || len(members) != 0 {
				t.Fatalf("empty HA group members must be []: %#v", result["members"])
			}
		},
		emptyResult:      "group contains a non-nil empty members array when HA is unconfigured",
		credentialPolicy: "group inspect omits peer secrets and identity credentials",
		edgeCases:        []string{"number", "boolean", "array", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{"ID: group-id", "Tag: ha-group", "Mode: automatic"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human baseline is missing %q: output=%q diagnostics=%q", expected, output, diagnostics)
				}
			}
		},
	}
}
