//go:build linux

package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dnsforwardcmd "github.com/NlightN22/xray-p2p/go/internal/cli/dnsforward"
	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/dnsforward"
)

type stage6DNSManager struct {
	mode             string
	diagnosticsCalls int
}

func (m *stage6DNSManager) Add(_ context.Context, opts dnsforward.AddOptions) (dnsforward.ListEntry, error) {
	if m.mode == "error" {
		return dnsforward.ListEntry{}, errors.New("platform DNS add failure \x1b[31m")
	}
	labels := []string{"xp2p", "forward:auto"}
	if m.mode == "control" {
		labels[1] = "forward:\n\t\u0001"
	}
	return dnsforward.ListEntry{
		Domain: opts.Domain, Target: opts.Target, Server: "127.0.0.1#5353",
		Labels: labels,
	}, nil
}

func (m *stage6DNSManager) Remove(opts dnsforward.RemoveOptions) ([]string, error) {
	if m.mode == "error" {
		return nil, errors.New("platform DNS remove failure \x1b[31m")
	}
	if opts.All {
		return []string{"alpha.example", "東京.example"}, nil
	}
	if m.mode == "control" {
		return []string{"control:\n\t\u0001"}, nil
	}
	return []string{opts.Domain}, nil
}

func (m *stage6DNSManager) List() ([]dnsforward.ListEntry, bool, error) {
	if m.mode == "error" {
		return nil, false, errors.New("platform DNS list failure \x1b[31m")
	}
	if m.mode == "empty" {
		return []dnsforward.ListEntry{}, false, nil
	}
	labels := []string{"xp2p", "forward:auto"}
	if m.mode == "control" {
		labels[1] = "forward:\n\t\u0001"
	}
	return []dnsforward.ListEntry{{
		Domain: "東京.example", Server: "127.0.0.1#5353",
		Labels: labels,
	}}, true, nil
}

func (m *stage6DNSManager) Diagnostics(bool) map[string]string {
	m.diagnosticsCalls++
	return map[string]string{"warning": "\x1b[33mplatform warning\x1b[0m"}
}

func registerStage6PlatformContractCases(registry map[string]contractCase) {
	for _, role := range []string{"client", "server"} {
		registry[fmt.Sprintf("xp2p %s dns-forward add", role)] = stage6DNSAddCase(role)
		registry[fmt.Sprintf("xp2p %s dns-forward list", role)] = stage6DNSListCase(role)
		registry[fmt.Sprintf("xp2p %s dns-forward remove", role)] = stage6DNSRemoveCase(role)
	}
	registry["xp2p nat-redirect add"] = stage6NATAddCase()
	registry["xp2p nat-redirect list"] = stage6NATListCase()
	registry["xp2p nat-redirect remove"] = stage6NATRemoveCase()
}

func stage6PlatformCasesExecutable() bool {
	return true
}

func TestStage6GateRejectsIncompletePlatformDescriptor(t *testing.T) {
	actual := jsonLeafPaths(NewCommand())
	expected := make(map[string]bool)
	for path, contract := range outputContractInventory {
		if contract.class == clioutput.ClassJSON {
			expected[path] = true
		}
	}
	registry := make(map[string]contractCase, len(contractCaseRegistry))
	for path, scenario := range contractCaseRegistry {
		registry[path] = scenario
	}
	path := "xp2p client dns-forward add"
	registry[path] = contractCase{
		coverage:     contractCovered,
		platformCase: true,
		platform:     "linux",
	}
	err := validateContractRegistry(actual, expected, registry)
	if err == nil || !strings.Contains(err.Error(), "covered case has incomplete scenarios: "+path) {
		t.Fatalf("incomplete platform descriptor was not rejected: %v", err)
	}
}

func stage6DNSAddCase(role string) contractCase {
	args := []string{role, "dns-forward", "add", "--domain", "東京.example", "--target", "192.0.2.53:5353"}
	return stage6Case(args, args, func(t *testing.T, mode string) {
		setupStage6DNS(t, mode)
	}, func(t *testing.T, result map[string]any) {
		labels, ok := result["labels"].([]any)
		if result["status"] != "completed" || result["domain"] != "東京.example" ||
			result["target"] != "192.0.2.53:5353" || result["server"] != "127.0.0.1#5353" ||
			!ok || len(labels) != 2 || labels[0] != "xp2p" || labels[1] != "forward:auto" {
			t.Fatalf("DNS add result=%#v", result)
		}
		assertNoCredentialFields(t, result, "dns_add")
	}, "idempotent add returns the same typed entry", "dns-forward added")
}

func stage6DNSListCase(role string) contractCase {
	args := []string{role, "dns-forward", "list"}
	scenario := stage6Case(args, args, func(t *testing.T, mode string) {
		setupStage6DNS(t, mode)
	}, func(t *testing.T, result map[string]any) {
		entries, ok := result["entries"].([]any)
		if !ok || len(entries) != 1 || result["intercept_enabled"] != true {
			t.Fatalf("DNS list result=%#v", result)
		}
		entry, ok := entries[0].(map[string]any)
		labels, labelsOK := entry["labels"].([]any)
		if !ok || entry["domain"] != "東京.example" || entry["server"] != "127.0.0.1#5353" ||
			!labelsOK || len(labels) != 2 || labels[0] != "xp2p" || labels[1] != "forward:auto" {
			t.Fatalf("DNS list entry=%#v", entries[0])
		}
		assertNoCredentialFields(t, result, "dns_list")
	}, "entries is a non-nil empty array and intercept_enabled is false", "東京.example")
	scenario.assertEmpty = func(t *testing.T, result map[string]any) {
		entries, ok := result["entries"].([]any)
		if !ok || entries == nil || len(entries) != 0 || result["intercept_enabled"] != false {
			t.Fatalf("DNS empty result=%#v", result)
		}
	}
	return scenario
}

func stage6DNSRemoveCase(role string) contractCase {
	args := []string{role, "dns-forward", "remove", "--domain", "東京.example"}
	return stage6Case(args, args, func(t *testing.T, mode string) {
		setupStage6DNS(t, mode)
	}, func(t *testing.T, result map[string]any) {
		domains, ok := result["domains"].([]any)
		if result["status"] != "completed" || !ok || len(domains) != 1 || domains[0] != "東京.example" {
			t.Fatalf("DNS remove result=%#v", result)
		}
		assertNoCredentialFields(t, result, "dns_remove")
	}, "idempotent remove returns the same typed domain list", "dns-forward removed")
}

func setupStage6DNS(t *testing.T, mode string) {
	t.Helper()
	stage6BaseSetup(t)
	manager := &stage6DNSManager{mode: mode}
	factory := func(config.Config) (dnsforwardcmd.Manager, error) { return manager, nil }
	restore := dnsforwardcmd.SetManagerFactoriesForTesting(factory, factory)
	t.Cleanup(restore)
}

func stage6NATAddCase() contractCase {
	return stage6NATPlanCase("add", []string{
		"nat-redirect", "add", "--cidr", "198.51.100.0/24", "--port", "12345", "--print-only",
	}, func(t *testing.T, result map[string]any) {
		assertStage6Plan(t, result, false, true)
	})
}

func stage6NATRemoveCase() contractCase {
	scenario := stage6NATPlanCase("remove", []string{
		"nat-redirect", "remove", "--cidr", "198.51.100.0/24", "--print-only",
	}, func(t *testing.T, result map[string]any) {
		assertStage6Plan(t, result, false, false)
	})
	scenario.assertHuman = func(t *testing.T, output, diagnostics string) {
		if !strings.Contains(output, "Planned iptables commands:") ||
			!strings.Contains(output, "Entry file would be written") ||
			!strings.Contains(diagnostics, "INFO xp2p starting") {
			t.Fatalf("human NAT remove output changed: stdout=%q stderr=%q", output, diagnostics)
		}
	}
	return scenario
}

func stage6NATPlanCase(action string, base []string, assert func(*testing.T, map[string]any)) contractCase {
	args := append(append([]string{}, base...), "--snippet", "", "--entry-dir", "")
	return stage6Case(args, args, func(t *testing.T, mode string) {
		root, snippet, entries := setupStage6NAT(t)
		if mode == "error" {
			entries = filepath.Join(root, "[")
		}
		args[len(args)-3] = snippet
		args[len(args)-1] = entries
	}, assert, "print-only returns the same typed plan without host changes", "table inet xray_transparent")
}

func stage6NATListCase() contractCase {
	args := []string{"nat-redirect", "list"}
	scenario := stage6Case(args, args, func(t *testing.T, mode string) {
		root, _, entries := setupStage6NAT(t)
		if mode == "success" {
			if err := os.MkdirAll(entries, 0o755); err != nil {
				t.Fatal(err)
			}
			body := "CIDR=\"198.51.100.0/24\"\nPORT=\"12345\"\n"
			if err := os.WriteFile(filepath.Join(entries, "redirect.entry"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if mode == "error" {
			t.Setenv("XP2P_CONFIG_ROOT", filepath.Join(root, "["))
		}
	}, func(t *testing.T, result map[string]any) {
		entries, ok := result["entries"].([]any)
		if !ok || len(entries) != 1 {
			t.Fatalf("NAT list result=%#v", result)
		}
		entry, ok := entries[0].(map[string]any)
		if !ok || entry["cidr"] != "198.51.100.0/24" || entry["port"] != float64(12345) {
			t.Fatalf("NAT list entry=%#v", entries[0])
		}
	}, "entries is a non-nil empty array", "198.51.100.0/24")
	scenario.assertEmpty = func(t *testing.T, result map[string]any) {
		entries, ok := result["entries"].([]any)
		if !ok || entries == nil || len(entries) != 0 {
			t.Fatalf("NAT empty result=%#v", result)
		}
	}
	return scenario
}

func stage6Case(
	success, human []string,
	setup func(*testing.T, string),
	assert func(*testing.T, map[string]any),
	emptyResult, humanText string,
) contractCase {
	return contractCase{
		coverage: contractCovered, success: success, empty: success, failure: success,
		setup: setup, assertResult: assert, assertEmpty: assert,
		emptyResult:      emptyResult,
		credentialPolicy: "platform network and firewall results omit credentials",
		edgeCases:        []string{"number", "boolean", "Unicode/control characters", "ANSI-free streams"},
		assertEdgeCases:  assertReadOnlyEdgeCases,
		platform:         "linux",
		human:            human,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			if !strings.Contains(output+diagnostics, humanText) {
				t.Fatalf("human output is missing %q: stdout=%q stderr=%q", humanText, output, diagnostics)
			}
		},
	}
}

func setupStage6NAT(t *testing.T) (root, snippet, entries string) {
	t.Helper()
	root = t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	if err := os.WriteFile(filepath.Join(root, "client.toml"), []byte("[client]\ntun_enabled = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, filepath.Join(root, "redirect.nft"), filepath.Join(root, "nftables", "xray-transparent.d")
}

func assertStage6Plan(t *testing.T, result map[string]any, removeAll, withEntry bool) {
	t.Helper()
	assertNoCredentialFields(t, result, "nat_plan")
	for _, field := range []string{"backend", "snippet", "snippet_path", "entry_path", "iptables", "remove_all", "use_fw4", "entry"} {
		if _, ok := result[field]; !ok {
			t.Fatalf("typed firewall plan is missing %q: %#v", field, result)
		}
	}
	if result["backend"] != "iptables" && result["backend"] != "nft" && result["backend"] != "fw4" {
		t.Fatalf("backend=%#v", result["backend"])
	}
	if _, ok := result["snippet"].(string); !ok {
		t.Fatalf("snippet type=%T", result["snippet"])
	}
	snippetPath, ok := result["snippet_path"].(string)
	if !ok || snippetPath != filepath.Join(os.Getenv("XP2P_CONFIG_ROOT"), "redirect.nft") {
		t.Fatalf("snippet_path type=%T", result["snippet_path"])
	}
	entryPath, ok := result["entry_path"].(string)
	if !ok || !strings.HasSuffix(filepath.ToSlash(entryPath), "/xray_redirect_198_51_100_0_24.entry") {
		t.Fatalf("entry_path type=%T", result["entry_path"])
	}
	commands, ok := result["iptables"].([]any)
	if !ok || len(commands) < 10 {
		t.Fatalf("iptables=%#v", result["iptables"])
	}
	for _, command := range commands {
		if _, ok := command.(string); !ok {
			t.Fatalf("iptables command type=%T", command)
		}
	}
	if result["remove_all"] != removeAll {
		t.Fatalf("remove_all=%#v", result["remove_all"])
	}
	useFW4, ok := result["use_fw4"].(bool)
	if !ok || useFW4 && result["backend"] != "fw4" {
		t.Fatalf("use_fw4 type=%T", result["use_fw4"])
	}
	entry, ok := result["entry"].(map[string]any)
	if withEntry && (!ok || entry["cidr"] != "198.51.100.0/24" || entry["port"] != float64(12345)) {
		t.Fatalf("entry=%#v", result["entry"])
	}
	if !withEntry && result["entry"] != nil {
		t.Fatalf("entry=%#v", result["entry"])
	}
	snippet := result["snippet"].(string)
	if withEntry && !strings.Contains(snippet, "table inet xray_transparent") {
		t.Fatalf("add snippet=%q", snippet)
	}
	if !withEntry && snippet != "" {
		t.Fatalf("remove snippet=%q", snippet)
	}
	for _, path := range []string{snippetPath, entryPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("print-only changed host state at %s: %v", path, err)
		}
	}
}

func stage6BaseSetup(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
}
