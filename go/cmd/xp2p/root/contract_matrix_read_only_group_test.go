package root

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func clientGroupListContractCase() contractCase {
	args := []string{"client", "group", "list"}
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
			fixture := `[client]
[[client.endpoints]]
hostname = "alpha.example"
tag = "alpha"
address = "192.0.2.1"
port = 443

[[client.endpoints]]
hostname = "zulu.example"
tag = "zulu"
address = "192.0.2.2"
port = 443

[[client.endpoint_groups]]
group_id = "group-zulu"
tag = "Zulu Ω"
members = ["zulu", "alpha"]
mode = "automatic"

[[client.endpoint_groups]]
group_id = "group-alpha"
tag = "Alpha group"
members = ["alpha"]
mode = "manual"
manual_active_tag = "alpha"
`
			writeContractFixture(t, filepath.Join(root, layout.ClientConfigFileName), fixture)
			liveDir := filepath.Join(root, layout.StateDirName, layout.LiveDirName, layout.ClientConfigDir)
			if err := os.MkdirAll(liveDir, 0o700); err != nil {
				t.Fatal(err)
			}
			selector := `not-json`
			if mode == "success" {
				selector = `{"revision":7,"groups":{"group-zulu":{"active_tag":"zulu","cooldown_until":"2026-07-24T10:20:30Z"},"group-alpha":{"active_tag":"alpha"}}}` + "\n"
			}
			writeContractFixture(t, filepath.Join(liveDir, layout.ClientEndpointSelectorStateFileName), selector)
		},
		assertResult: func(t *testing.T, result map[string]any) {
			groups, ok := result["groups"].([]any)
			if !ok || len(groups) != 2 {
				t.Fatalf("groups=%#v", result["groups"])
			}
			zulu, ok := groups[0].(map[string]any)
			if !ok || zulu["group_id"] != "group-zulu" || zulu["tag"] != "Zulu Ω" ||
				zulu["active_tag"] != "zulu" || zulu["revision"] != float64(7) ||
				zulu["cooldown_until"] != "2026-07-24T10:20:30Z" {
				t.Fatalf("first group changed: %#v", groups[0])
			}
			members, ok := zulu["members"].([]any)
			if !ok || len(members) != 2 || members[0] != "zulu" || members[1] != "alpha" {
				t.Fatalf("members must preserve their typed order: %#v", zulu["members"])
			}
			alpha, ok := groups[1].(map[string]any)
			if !ok || alpha["group_id"] != "group-alpha" || alpha["mode"] != "manual" {
				t.Fatalf("group order or mode changed: %#v", groups[1])
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("group list leaked incidental credentials: %#v", result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			groups, ok := result["groups"].([]any)
			if !ok || groups == nil || len(groups) != 0 {
				t.Fatalf("empty groups must be []: %#v", result["groups"])
			}
		},
		emptyResult:      "groups is a non-nil empty array when no groups exist",
		credentialPolicy: "list omits endpoint credentials and private state",
		edgeCases:        []string{"number", "array", "UTC RFC3339", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{
				"GROUP ID", "TAG", "MODE", "ACTIVE", "MEMBERS", "COOLDOWN", "REVISION",
				"group-zulu", "Zulu Ω", "zulu,alpha", "2026-07-24T10:20:30Z", "7",
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
