//go:build linux

package root

import (
	"context"
	"encoding/json"
	"errors"
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
	fail bool
}

func (m *stage6DNSManager) Add(_ context.Context, opts dnsforward.AddOptions) (dnsforward.ListEntry, error) {
	if m.fail {
		return dnsforward.ListEntry{}, errors.New("platform DNS add failure \x1b[31m")
	}
	return dnsforward.ListEntry{
		Domain: opts.Domain, Target: opts.Target, Server: "127.0.0.1#5353",
		Labels: []string{"xp2p", "forward:auto"},
	}, nil
}

func (m *stage6DNSManager) Remove(opts dnsforward.RemoveOptions) ([]string, error) {
	if m.fail {
		return nil, errors.New("platform DNS remove failure \x1b[31m")
	}
	if opts.All {
		return []string{"alpha.example", "東京.example"}, nil
	}
	return []string{opts.Domain}, nil
}

func (m *stage6DNSManager) List() ([]dnsforward.ListEntry, bool, error) {
	if m.fail {
		return nil, false, errors.New("platform DNS list failure \x1b[31m")
	}
	return []dnsforward.ListEntry{{
		Domain: "東京.example", Server: "127.0.0.1#5353",
		Labels: []string{"xp2p", "forward:auto"},
	}}, true, nil
}

func (*stage6DNSManager) Diagnostics(bool) map[string]string {
	return map[string]string{"warning": "\x1b[33mplatform warning\x1b[0m"}
}

func TestStage6DNSForwardContracts(t *testing.T) {
	for _, role := range []string{"client", "server"} {
		role := role
		t.Run(role, func(t *testing.T) {
			manager := &stage6DNSManager{}
			factory := func(config.Config) (dnsforwardcmd.Manager, error) { return manager, nil }
			restore := dnsforwardcmd.SetManagerFactoriesForTesting(factory, factory)
			t.Cleanup(restore)

			add := executeContractCase([]string{
				role, "dns-forward", "add", "--domain", "東京.example",
				"--target", "192.0.2.53:5353",
			}, false)
			result := stage6Result(t, "xp2p "+role+" dns-forward add", add)
			if result["status"] != "completed" || result["domain"] != "東京.example" ||
				result["target"] != "192.0.2.53:5353" {
				t.Fatalf("add result=%#v", result)
			}
			assertNoCredentialFields(t, result, "result")

			list := executeContractCase([]string{role, "dns-forward", "list"}, false)
			result = stage6Result(t, "xp2p "+role+" dns-forward list", list)
			if result["intercept_enabled"] != true {
				t.Fatalf("list result=%#v", result)
			}

			remove := executeContractCase([]string{
				role, "dns-forward", "remove", "--domain", "東京.example",
			}, false)
			result = stage6Result(t, "xp2p "+role+" dns-forward remove", remove)
			domains, ok := result["domains"].([]any)
			if result["status"] != "completed" || !ok || len(domains) != 1 || domains[0] != "東京.example" {
				t.Fatalf("remove result=%#v", result)
			}

			manager.fail = true
			for _, args := range [][]string{
				{role, "dns-forward", "add", "--domain", "error.example", "--target", "192.0.2.1:53"},
				{role, "dns-forward", "list"},
				{role, "dns-forward", "remove", "--domain", "error.example"},
			} {
				execution := executeContractCase(args, false)
				assertStage6Error(t, "xp2p "+strings.Join(args[:len(args)-flagTail(args)], " "), execution)
			}
		})
	}
}

func TestStage6NATPrintOnlyContracts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	configBody := "[client]\ntun_enabled = false\n"
	if err := os.WriteFile(filepath.Join(root, "client.toml"), []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}
	snippet := filepath.Join(root, "redirect.nft")
	entries := filepath.Join(root, "entries")
	add := executeContractCase([]string{
		"nat-redirect", "add", "--cidr", "198.51.100.0/24", "--port", "12345",
		"--print-only", "--snippet", snippet, "--entry-dir", entries,
	}, false)
	result := stage6Result(t, "xp2p nat-redirect add", add)
	entry, ok := result["entry"].(map[string]any)
	if !ok || entry["cidr"] != "198.51.100.0/24" || entry["port"] != float64(12345) ||
		result["remove_all"] != false {
		t.Fatalf("add plan=%#v", result)
	}
	if _, err := os.Stat(snippet); !os.IsNotExist(err) {
		t.Fatalf("print-only changed host state: %v", err)
	}

	remove := executeContractCase([]string{
		"nat-redirect", "remove", "--all", "--print-only",
		"--snippet", snippet, "--entry-dir", entries,
	}, false)
	result = stage6Result(t, "xp2p nat-redirect remove", remove)
	if result["remove_all"] != true || result["entry"] != nil {
		t.Fatalf("remove plan=%#v", result)
	}

	assertStage6Error(t, "xp2p nat-redirect add",
		executeContractCase([]string{"nat-redirect", "add", "--cidr", "invalid", "--port", "12345", "--print-only"}, false))
}

func stage6Result(t *testing.T, path string, execution contractExecution) map[string]any {
	t.Helper()
	if execution.exitCode != 0 || execution.err != nil || execution.stderr != "" {
		t.Fatalf("%s: exit=%d err=%v stdout=%q stderr=%q", path, execution.exitCode, execution.err, execution.stdout, execution.stderr)
	}
	document := assertJSONDocument(t, execution.stdout)
	var envelope struct {
		SchemaVersion string         `json:"schema_version"`
		Command       string         `json:"command"`
		Result        map[string]any `json:"result"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != clioutput.SchemaVersion || envelope.Command != path {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if strings.Contains(execution.stdout, "\x1b[") {
		t.Fatalf("ANSI leaked: %q", execution.stdout)
	}
	return envelope.Result
}

func assertStage6Error(t *testing.T, path string, execution contractExecution) {
	t.Helper()
	assertStage5Error(t, path, execution)
	if strings.Contains(execution.stderr, "\x1b[") {
		t.Fatalf("ANSI leaked: %q", execution.stderr)
	}
}

func flagTail(args []string) int {
	for index, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return len(args) - index
		}
	}
	return 0
}
