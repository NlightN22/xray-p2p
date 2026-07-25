//go:build linux

package root

import (
	"encoding/json"
	"strings"
	"testing"

	dnsforwardcmd "github.com/NlightN22/xray-p2p/go/internal/cli/dnsforward"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestStage6DNSWarningsExecuteDiagnosticsWithoutLeaking(t *testing.T) {
	for _, role := range []string{"client", "server"} {
		for _, item := range []struct {
			action string
			args   []string
		}{
			{"add", []string{role, "dns-forward", "add", "--domain", "error.example", "--target", "192.0.2.1:53", "--debug"}},
			{"list", []string{role, "dns-forward", "list", "--debug"}},
			{"remove", []string{role, "dns-forward", "remove", "--domain", "error.example", "--debug"}},
		} {
			manager := &stage6DNSManager{mode: "error"}
			factory := func(config.Config) (dnsforwardcmd.Manager, error) { return manager, nil }
			restore := dnsforwardcmd.SetManagerFactoriesForTesting(factory, factory)
			t.Cleanup(restore)

			path := "xp2p " + role + " dns-forward " + item.action
			execution := executeContractCase(item.args, false)
			assertStage5Error(t, path, execution)
			if manager.diagnosticsCalls != 1 {
				t.Fatalf("%s: diagnostics calls=%d", path, manager.diagnosticsCalls)
			}
			if strings.Contains(execution.stderr, "\x1b[") ||
				strings.Contains(execution.stderr, "platform warning") {
				t.Fatalf("%s leaked warning diagnostics: %q", path, execution.stderr)
			}
		}
	}
}

func TestStage6DNSControlCharactersUseJSONEscaping(t *testing.T) {
	manager := &stage6DNSManager{mode: "control"}
	factory := func(config.Config) (dnsforwardcmd.Manager, error) { return manager, nil }
	restore := dnsforwardcmd.SetManagerFactoriesForTesting(factory, factory)
	t.Cleanup(restore)

	for _, item := range []struct {
		path string
		args []string
		key  string
	}{
		{"xp2p client dns-forward add", []string{"client", "dns-forward", "add", "--domain", "control.example", "--target", "192.0.2.1:53"}, "labels"},
		{"xp2p client dns-forward list", []string{"client", "dns-forward", "list"}, "entries"},
		{"xp2p client dns-forward remove", []string{"client", "dns-forward", "remove", "--domain", "control.example"}, "domains"},
	} {
		execution := executeContractCase(item.args, false)
		result := stage6ResultMap(t, item.path, execution)
		if !strings.Contains(execution.stdout, `\n\t\u0001`) ||
			strings.Contains(execution.stdout, "forward:\n") {
			t.Fatalf("%s did not JSON-escape control characters: %q", item.path, execution.stdout)
		}
		encoded, err := json.Marshal(result[item.key])
		if err != nil || !strings.Contains(string(encoded), `\n\t\u0001`) {
			t.Fatalf("%s lost control characters: value=%#v err=%v", item.path, result[item.key], err)
		}
	}
}

func stage6ResultMap(t *testing.T, path string, execution contractExecution) map[string]any {
	t.Helper()
	if execution.exitCode != 0 || execution.err != nil || execution.stderr != "" {
		t.Fatalf("%s: exit=%d err=%v stdout=%q stderr=%q",
			path, execution.exitCode, execution.err, execution.stdout, execution.stderr)
	}
	document := assertJSONDocument(t, execution.stdout)
	var envelope struct {
		Command string         `json:"command"`
		Result  map[string]any `json:"result"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Command != path {
		t.Fatalf("command=%q want=%q", envelope.Command, path)
	}
	return envelope.Result
}
