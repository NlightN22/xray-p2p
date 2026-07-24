package root

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func redirectListContractCase(role string) contractCase {
	args := []string{role, "redirect", "list"}
	fileName := layout.ClientConfigFileName
	collectionKey := "redirects"
	if role == "server" {
		args = append(args, "--pending")
		fileName = layout.ServerConfigFileName
		collectionKey = "server_redirects"
	}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup: func(t *testing.T, mode string) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			header := fmt.Sprintf("[%s]\n", role)
			if role == "server" {
				header += fmt.Sprintf("install_dir = %q\n", safeServerInstallDir())
			}
			if mode == "empty" {
				if role == "server" {
					writeContractFixture(t, filepath.Join(root, fileName), header)
				}
				return
			}
			tablePath := role + "." + collectionKey
			fixture := header + fmt.Sprintf("%s = \"invalid\"\n", collectionKey)
			if mode == "success" {
				fixture = header + fmt.Sprintf(`
[[%s]]
domain = "zulu Ω.example"
outbound_tag = "edge-zulu"
disabled = true

[[%s]]
cidr = "192.0.2.0/24"
outbound_tag = "edge-alpha"
`, tablePath, tablePath)
				if role == "client" {
					fixture += `
[[client.endpoints]]
hostname = "alpha host"
tag = "edge-alpha"

[[client.endpoints]]
hostname = "zulu host"
tag = "edge-zulu"
`
				} else {
					fixture += `
[server.reverse_channels.alpha]
host = "alpha host"
tag = "edge-alpha"

[server.reverse_channels.zulu]
host = "zulu host"
tag = "edge-zulu"
`
				}
			}
			writeContractFixture(t, filepath.Join(root, fileName), fixture)
		},
		assertResult: func(t *testing.T, result map[string]any) {
			redirects, ok := result["redirects"].([]any)
			if !ok || len(redirects) != 2 {
				t.Fatalf("redirects=%#v", result["redirects"])
			}
			domain, ok := redirects[0].(map[string]any)
			if !ok || domain["type"] != "domain" || domain["value"] != "zulu ω.example" ||
				domain["host"] != "zulu host" || domain["enabled"] != false {
				t.Fatalf("domain redirect changed: %#v", redirects[0])
			}
			cidr, ok := redirects[1].(map[string]any)
			if !ok || cidr["type"] != "CIDR" || cidr["value"] != "192.0.2.0/24" ||
				cidr["host"] != "alpha host" || cidr["enabled"] != true {
				t.Fatalf("CIDR redirect changed: %#v", redirects[1])
			}
			if role == "server" {
				if _, ok := domain["disabled_by_policy"].(bool); !ok {
					t.Fatalf("disabled_by_policy must be boolean: %#v", domain)
				}
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("%s redirect list leaked incidental credentials: %#v", role, result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			redirects, ok := result["redirects"].([]any)
			if !ok || redirects == nil || len(redirects) != 0 {
				t.Fatalf("empty redirects must be []: %#v", result["redirects"])
			}
		},
		emptyResult:      "redirects is a non-nil empty array when no rules exist",
		credentialPolicy: "list omits passwords, tokens, and private keys",
		edgeCases:        []string{"boolean", "ordered collection", "Unicode/spaces", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{
				"TYPE", "VALUE", "OUTBOUND TAG", "HOST", "STATE",
				"zulu ω.example", "192.0.2.0/24", "zulu host", "disabled",
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

func writeContractFixture(t *testing.T, path, fixture string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
}
