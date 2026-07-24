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
id = "zulu Ω"
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
			if !ok || zulu["id"] != "zulu Ω" || zulu["endpoint"] != "https://zulu.example:8443" ||
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
