package root

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func modeReadContractCase(role string) contractCase {
	args := []string{role, "mode"}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup: func(t *testing.T, mode string) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			configName := layout.ServerConfigFileName
			stateName := layout.ServerAppliedStateFileName
			fixture := "[server]\n"
			if role == "client" {
				configName = layout.ClientConfigFileName
				stateName = layout.ClientAppliedStateFileName
				fixture = "[client]\n"
			}
			if mode == "success" {
				if role == "client" {
					fixture = "[client]\ntun_enabled = true\ntun_mode = \"full\"\n"
				} else {
					fixture = "[server]\ntun_enabled = true\n"
				}
				writeContractFixture(t, filepath.Join(root, stateName), `{"mode":"tun","tun_enabled":true}`)
			} else if mode == "empty" {
				writeContractFixture(t, filepath.Join(root, stateName), `{"mode":"proxy","tun_enabled":false}`)
			} else if mode == "error" {
				writeContractFixture(t, filepath.Join(root, stateName), "{invalid")
			}
			writeContractFixture(t, filepath.Join(root, configName), fixture)
		},
		assertResult: func(t *testing.T, result map[string]any) {
			if result["mode"] != "tun" {
				t.Fatalf("%s mode changed: %#v", role, result)
			}
			if role == "client" {
				if result["tun_mode"] != "full" || result["tun_mode_status"] != "pending" {
					t.Fatalf("client TUN mode fields changed: %#v", result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			if result["mode"] != "proxy" {
				t.Fatalf("default %s mode changed: %#v", role, result)
			}
			if role == "client" &&
				(result["tun_mode"] != "" || result["tun_mode_status"] != "") {
				t.Fatalf("proxy mode must keep empty TUN fields: %#v", result)
			}
		},
		emptyResult:      "proxy applied state keeps client TUN-only fields empty",
		credentialPolicy: "mode status contains no credentials",
		edgeCases:        []string{"default value", "malformed applied state", "warning isolation", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			if output != "" {
				t.Fatalf("human mode query unexpectedly wrote stdout: %q", output)
			}
			for _, expected := range []string{"current mode", "tun"} {
				if !strings.Contains(diagnostics, expected) {
					t.Fatalf("human mode diagnostic is missing %q: %q", expected, diagnostics)
				}
			}
		},
	}
}
