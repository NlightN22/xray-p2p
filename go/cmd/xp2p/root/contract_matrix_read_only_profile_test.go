package root

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func serverProfileContractCase() contractCase {
	args := []string{"server", "profile"}
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
			if mode == "success" {
				fixture = "[server]\nprofile = \"vless-tls-vision\"\n"
			} else if mode == "error" {
				fixture = "[server]\nprofile = \"unknown Ω profile\"\n"
			}
			writeContractFixture(t, filepath.Join(root, layout.ServerConfigFileName), fixture)
		},
		assertResult: func(t *testing.T, result map[string]any) {
			if result["profile"] != "vless-tls-vision" {
				t.Fatalf("server profile changed: %#v", result)
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			if result["profile"] != "trojan-tls" {
				t.Fatalf("default server profile changed: %#v", result)
			}
		},
		emptyResult:      "an omitted profile resolves to the backward-compatible trojan-tls default",
		credentialPolicy: "profile contains no credentials",
		edgeCases:        []string{"default value", "invalid persisted enum", "Unicode/spaces in error", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			if output != "" {
				t.Fatalf("human profile query unexpectedly wrote stdout: %q", output)
			}
			if !strings.Contains(diagnostics, "current profile") || !strings.Contains(diagnostics, "vless-tls-vision") {
				t.Fatalf("human profile diagnostic changed: %q", diagnostics)
			}
		},
	}
}
