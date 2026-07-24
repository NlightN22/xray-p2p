package root

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func forwardListContractCase(role string) contractCase {
	path := "xp2p " + role + " forward list"
	args := []string{role, "forward", "list"}
	fileName := layout.ClientConfigFileName
	collectionKey := "forwards"
	if role == "server" {
		args = append(args, "--pending")
		fileName = layout.ServerConfigFileName
		collectionKey = "forward_rules"
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
			if mode == "empty" {
				if role == "server" {
					fixture := fmt.Sprintf("[server]\ninstall_dir = %q\n", safeServerInstallDir())
					if err := os.WriteFile(filepath.Join(root, fileName), []byte(fixture), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				return
			}
			header := fmt.Sprintf("[%s]\n", role)
			if role == "server" {
				header += fmt.Sprintf("install_dir = %q\n", safeServerInstallDir())
			}
			tablePath := role + "." + collectionKey
			fixture := header + fmt.Sprintf(`
[[%s]]
listen_address = "127.0.0.1"
listen_port = "invalid"
`, tablePath)
			if mode == "success" {
				fixture = header + fmt.Sprintf(`
[[%s]]
listen_address = "127.0.0.2"
listen_port = 62002
target_host = "zulu.example"
target_port = 443
protocol = "both"
tag = "zulu"
remark = "Zulu Ω"

[[%s]]
listen_address = "127.0.0.1"
listen_port = 61001
target_host = "alpha example"
target_port = 53
protocol = "udp"
tag = "alpha"
remark = "Alpha\u0001"
`, tablePath, tablePath)
			}
			if err := os.WriteFile(filepath.Join(root, fileName), []byte(fixture), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		assertResult: func(t *testing.T, result map[string]any) {
			forwards, ok := result["forwards"].([]any)
			if !ok || len(forwards) != 2 {
				t.Fatalf("forwards=%#v", result["forwards"])
			}
			first, ok := forwards[0].(map[string]any)
			if !ok || first["listen_port"] != float64(62002) ||
				first["target"] != "zulu.example:443" || first["remark"] != "Zulu Ω" {
				t.Fatalf("first forward JSON types changed: %#v", forwards[0])
			}
			protocols, ok := first["protocols"].([]any)
			if !ok || len(protocols) != 2 || protocols[0] != "tcp" || protocols[1] != "udp" {
				t.Fatalf("protocols must be a typed array: %#v", first["protocols"])
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("%s leaked incidental credentials: %#v", path, result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			forwards, ok := result["forwards"].([]any)
			if !ok || forwards == nil || len(forwards) != 0 {
				t.Fatalf("empty forwards must be []: %#v", result["forwards"])
			}
		},
		emptyResult:      "forwards is a non-nil empty array when no Desired file exists",
		credentialPolicy: "list omits passwords, tokens, and private keys",
		edgeCases:        []string{"number", "array", "Unicode/control characters", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{
				"LISTEN", "PROTOCOLS", "TARGET", "REMARK", "127.0.0.2:62002",
				"tcp,udp", "zulu.example:443", "Zulu Ω",
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

func safeServerInstallDir() string {
	if runtime.GOOS == "windows" {
		return "C:/xp2p"
	}
	return "/opt/xp2p"
}

func clientReverseListContractCase() contractCase {
	return contractCase{
		coverage: contractCovered,
		success:  []string{"client", "reverse", "list"},
		empty:    []string{"client", "reverse", "list"},
		failure:  []string{"client", "reverse", "list"},
		setup: func(t *testing.T, mode string) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			if mode == "empty" {
				return
			}
			fixture := `[client]
[[client.endpoints]]
hostname = "edge.example"
tag = "primary"
heartbeat_mode = "unsupported"
`
			if mode == "success" {
				fixture = `[client.reverse.zulu]
tag = "zulu"
host = "zulu.example"
user_id = "user-zulu"
endpoint_tag = "edge-zulu"
disabled = true

[client.reverse.alpha]
tag = "alpha Ω"
host = "alpha example"
user_id = "user\u0001"
endpoint_tag = "edge-alpha"
`
			}
			path := filepath.Join(root, layout.ClientConfigFileName)
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
			if !ok || alpha["tag"] != "alpha Ω" || alpha["host"] != "alpha example" ||
				alpha["routing_bridge_present"] != true ||
				alpha["direct_rule_present"] != true || alpha["enabled"] != true {
				t.Fatalf("first reverse tunnel changed: %#v", tunnels[0])
			}
			zulu, ok := tunnels[1].(map[string]any)
			if !ok || zulu["tag"] != "zulu" || zulu["enabled"] != false {
				t.Fatalf("reverse tunnel order or boolean types changed: %#v", tunnels[1])
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("reverse list leaked incidental credentials: %#v", result)
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
		human:            []string{"client", "reverse", "list"},
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{
				"TAG", "HOST", "USER", "ENDPOINT TAG", "ROUTING-BRIDGE", "DIRECT RULE", "STATE",
				"alpha Ω", "zulu", "present", "disabled",
			} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human baseline is missing %q: %q", expected, output)
				}
			}
			if !strings.Contains(diagnostics, "INFO xp2p starting") {
				t.Fatalf("human diagnostic baseline changed: %q", diagnostics)
			}
		},
	}
}
