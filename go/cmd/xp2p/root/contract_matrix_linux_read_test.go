package root

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func dnsForwardListContractCase(role string) contractCase {
	args := []string{role, "dns-forward", "list"}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup: func(t *testing.T, mode string) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			t.Setenv("XP2P_DNSFORWARD_CONFIG", filepath.Join(root, "dnsmasq"))
			t.Setenv("XP2P_DNSFORWARD_FORCE_OPENWRT", "1")

			bin := filepath.Join(root, "bin")
			if err := os.MkdirAll(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			uci := filepath.Join(bin, "uci")
			if err := os.WriteFile(uci, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

			configName := layout.ClientConfigFileName
			if role == "server" {
				configName = layout.ServerConfigFileName
			}
			configBody := "[" + role + "]\ninstall_dir = " + tomlQuote(root) + "\n"
			if err := os.WriteFile(filepath.Join(root, configName), []byte(configBody), 0o600); err != nil {
				t.Fatal(err)
			}
			statePath := filepath.Join(root, "dns-forward-state.json")
			switch mode {
			case "success":
				state := `{"entries":{"zulu.example":{"target":"192.0.2.53:53","server":"127.0.0.1#5335","forward_listen_port":5335,"forward_owner":"dns-forward"}}}`
				if err := os.WriteFile(statePath, []byte(state), 0o600); err != nil {
					t.Fatal(err)
				}
			case "error":
				if err := os.WriteFile(statePath, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		},
		assertResult: func(t *testing.T, result map[string]any) {
			entries, ok := result["entries"].([]any)
			if !ok || len(entries) != 1 {
				t.Fatalf("entries=%#v", result["entries"])
			}
			entry, ok := entries[0].(map[string]any)
			if !ok || entry["domain"] != "zulu.example" ||
				entry["server"] != "127.0.0.1#5335" {
				t.Fatalf("entry=%#v", entries[0])
			}
			labels, ok := entry["labels"].([]any)
			if !ok || len(labels) != 2 ||
				labels[0] != "xp2p" || labels[1] != "forward:auto" {
				t.Fatalf("labels=%#v", entry["labels"])
			}
			if result["intercept_enabled"] != false {
				t.Fatalf("intercept_enabled=%#v", result["intercept_enabled"])
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			entries, ok := result["entries"].([]any)
			if !ok || entries == nil || len(entries) != 0 {
				t.Fatalf("empty entries must be []: %#v", result["entries"])
			}
			if result["intercept_enabled"] != false {
				t.Fatalf("intercept_enabled=%#v", result["intercept_enabled"])
			}
		},
		emptyResult:      "entries is a non-nil empty array and intercept_enabled is false",
		credentialPolicy: "DNS forwarding state contains no credentials",
		edgeCases:        []string{"boolean", "non-nil empty array", "ANSI-free streams"},
		platform:         "linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{"DOMAIN", "SERVER", "LABELS", "zulu.example", "127.0.0.1#5335", "forward:auto"} {
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

func natRedirectListContractCase() contractCase {
	args := []string{"nat-redirect", "list"}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup: func(t *testing.T, mode string) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			if mode == "success" {
				entryDir := filepath.Join(root, "nftables", "xray-transparent.d")
				if err := os.MkdirAll(entryDir, 0o755); err != nil {
					t.Fatal(err)
				}
				entryPath := filepath.Join(entryDir, "xray_redirect_198_51_100_0_24.entry")
				if err := os.WriteFile(entryPath, []byte("CIDR=\"198.51.100.0/24\"\nPORT=\"12345\"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "error" {
				if err := os.WriteFile(filepath.Join(root, layout.ClientConfigFileName), []byte("[client\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		},
		assertResult: func(t *testing.T, result map[string]any) {
			entries, ok := result["entries"].([]any)
			if !ok || len(entries) != 1 {
				t.Fatalf("entries=%#v", result["entries"])
			}
			entry, ok := entries[0].(map[string]any)
			if !ok || entry["cidr"] != "198.51.100.0/24" || entry["port"] != float64(12345) {
				t.Fatalf("entry=%#v", entries[0])
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			entries, ok := result["entries"].([]any)
			if !ok || entries == nil || len(entries) != 0 {
				t.Fatalf("empty entries must be []: %#v", result["entries"])
			}
		},
		emptyResult:      "entries is a non-nil empty array",
		credentialPolicy: "NAT redirect state contains no credentials",
		edgeCases:        []string{"number", "non-nil empty array", "ANSI-free streams"},
		platform:         "linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{"CIDR", "Port", "198.51.100.0/24", "12345"} {
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

func tomlQuote(value string) string {
	return `"` + strings.ReplaceAll(filepath.ToSlash(value), `"`, `\"`) + `"`
}
