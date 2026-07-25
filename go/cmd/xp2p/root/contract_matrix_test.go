package root

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

func TestContractCaseRegistryMatchesJSONLeaves(t *testing.T) {
	actual := jsonLeafPaths(NewCommand())
	expected := make(map[string]bool)
	for path, contract := range outputContractInventory {
		if contract.class == clioutput.ClassJSON {
			expected[path] = true
		}
	}
	if err := validateContractRegistry(actual, expected, contractCaseRegistry); err != nil {
		t.Fatal(err)
	}
	if err := validateStage4Contracts(
		stage4ExpectedPaths(),
		contractCaseRegistry,
		stage4ContractRegistry,
	); err != nil {
		t.Fatal(err)
	}
}

func TestContractCaseRegistryDetectsMissingAndStaleCases(t *testing.T) {
	cases := map[string]contractCase{"xp2p client list": {coverage: contractCovered}}
	if err := validateContractRegistry(
		map[string]bool{"xp2p client list": true, "xp2p client new": true},
		map[string]bool{"xp2p client list": true, "xp2p client new": true},
		cases,
	); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing case was not detected: %v", err)
	}
	if err := validateContractRegistry(
		map[string]bool{"xp2p client list": true},
		map[string]bool{"xp2p client list": true},
		map[string]contractCase{
			"xp2p client list": {coverage: contractCovered},
			"xp2p stale":       {coverage: contractCovered},
		},
	); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale case was not detected: %v", err)
	}
	if err := validateContractRegistry(
		map[string]bool{"xp2p client new": true},
		map[string]bool{"xp2p client new": true},
		map[string]contractCase{"xp2p client new": {coverage: "invalid"}},
	); err == nil || !strings.Contains(err.Error(), "invalid coverage status") {
		t.Fatalf("non-covered case was not rejected: %v", err)
	}
}

func TestStage2ContractCasesCovered(t *testing.T) {
	for path := range jsonLeafPaths(NewCommand()) {
		if !isStage2ReadOnlyLeaf(path) {
			continue
		}
		scenario, exists := contractCaseRegistry[path]
		if !exists || scenario.coverage != contractCovered {
			t.Errorf("read-only leaf %s is not covered in stage 2", path)
		}
	}
}

func isStage2ReadOnlyLeaf(path string) bool {
	parts := strings.Fields(path)
	if len(parts) == 0 {
		return false
	}
	switch parts[len(parts)-1] {
	case "contract", "inspect", "list", "mode", "obs", "offers", "profile", "state", "status":
		return true
	default:
		return false
	}
}

func TestCoveredContractCases(t *testing.T) {
	actual := jsonLeafPaths(NewCommand())
	for path, scenario := range contractCaseRegistry {
		if scenario.coverage != contractCovered {
			continue
		}
		if scenario.mutation {
			continue
		}
		if scenario.artifact {
			continue
		}
		if scenario.platformCase && !stage6PlatformCasesExecutable() {
			continue
		}
		if !actual[path] {
			continue
		}
		t.Run(path+"/success", func(t *testing.T) {
			scenario.setup(t, "success")
			execution := executeContractCase(scenario.success, false)
			if execution.exitCode != 0 {
				t.Fatalf("exit=%d err=%v; stderr=%q", execution.exitCode, execution.err, execution.stderr)
			}
			if execution.stderr != "" {
				t.Fatalf("stderr=%q", execution.stderr)
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
			scenario.assertResult(t, envelope.Result)
			scenario.assertEdgeCases(t, envelope.Result, execution.stdout, execution.stderr)
		})
		t.Run(path+"/empty", func(t *testing.T) {
			scenario.setup(t, "empty")
			execution := executeContractCase(scenario.empty, false)
			if execution.exitCode != 0 {
				t.Fatalf("exit=%d err=%v; stderr=%q", execution.exitCode, execution.err, execution.stderr)
			}
			if execution.stderr != "" {
				t.Fatalf("empty-result stderr=%q", execution.stderr)
			}
			document := assertJSONDocument(t, execution.stdout)
			var envelope struct {
				Result map[string]any `json:"result"`
			}
			if decodeErr := json.Unmarshal(document, &envelope); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			scenario.assertEmpty(t, envelope.Result)
			scenario.assertEdgeCases(t, envelope.Result, execution.stdout, execution.stderr)
		})
		t.Run(path+"/error", func(t *testing.T) {
			scenario.setup(t, "error")
			execution := executeContractCase(scenario.failure, scenario.cancelFailure)
			if execution.err == nil {
				t.Fatal("expected handler error")
			}
			if execution.exitCode == 0 {
				t.Fatalf("handler error has a zero process exit code: %T %v", execution.err, execution.err)
			}
			if execution.stdout != "" {
				t.Fatalf("stdout=%q", execution.stdout)
			}
			document := assertJSONDocument(t, execution.stderr)
			var envelope clioutput.ErrorEnvelope
			if decodeErr := json.Unmarshal(document, &envelope); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if envelope.SchemaVersion != clioutput.SchemaVersion ||
				envelope.Command != path || envelope.Error.Code != "command_failed" {
				t.Fatalf("unexpected error envelope: %#v", envelope)
			}
			if strings.Contains(execution.stderr, "\x1b[") {
				t.Fatalf("stderr leaked diagnostics: %q", execution.stderr)
			}
		})
		t.Run(path+"/diagnostic", func(t *testing.T) {
			scenario.setup(t, "error")
			execution := executeContractCase(scenario.failure, scenario.cancelFailure)
			if execution.exitCode == 0 || execution.err == nil || execution.stdout != "" {
				t.Fatalf("diagnostic path contract changed: exit=%d err=%v stdout=%q", execution.exitCode, execution.err, execution.stdout)
			}
			document := assertJSONDocument(t, execution.stderr)
			var envelope clioutput.ErrorEnvelope
			if err := json.Unmarshal(document, &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Command != path || envelope.Error.Code != "command_failed" ||
				strings.Contains(execution.stderr, "\x1b[") {
				t.Fatalf("diagnostic path leaked outside its error envelope: %#v stderr=%q", envelope, execution.stderr)
			}
		})
		t.Run(path+"/human", func(t *testing.T) {
			scenario.setup(t, "success")
			stdout, stderr, err := executeHumanContractCase(scenario.human)
			if err != nil {
				t.Fatalf("execute: %v; stderr=%q", err, stderr)
			}
			scenario.assertHuman(t, stdout, stderr)
			assertHumanBaseline(t, path, stdout, stderr)
		})
	}
}

func validateContractRegistry(actual, expected map[string]bool, registry map[string]contractCase) error {
	var problems []string
	for path := range actual {
		if !expected[path] {
			problems = append(problems, "JSON leaf absent from output inventory: "+path)
		}
	}
	for path := range expected {
		if !actual[path] && !platformSpecificOutputContracts[path] {
			problems = append(problems, "inventory JSON leaf absent from Cobra tree: "+path)
		}
		if _, ok := registry[path]; !ok {
			problems = append(problems, "missing contract case: "+path)
		}
	}
	for path, scenario := range registry {
		if !expected[path] {
			problems = append(problems, "stale contract case: "+path)
		}
		if scenario.coverage != contractCovered {
			problems = append(problems, "invalid coverage status: "+path)
		}
		requiresExecutableMatrix := scenario.coverage == contractCovered &&
			!scenario.mutation && !scenario.artifact &&
			(!scenario.platformCase || stage6PlatformCasesExecutable())
		if requiresExecutableMatrix &&
			(len(scenario.success) == 0 || len(scenario.empty) == 0 ||
				len(scenario.failure) == 0 ||
				scenario.setup == nil || scenario.assertResult == nil ||
				scenario.assertEmpty == nil ||
				scenario.emptyResult == "" || scenario.credentialPolicy == "" ||
				scenario.assertEdgeCases == nil || scenario.platform == "" ||
				len(scenario.human) == 0 || scenario.assertHuman == nil) {
			problems = append(problems, "covered case has incomplete scenarios: "+path)
		}
		if requiresExecutableMatrix && humanBaselineDigests[path] == "" {
			problems = append(problems, "covered case has no exact human baseline: "+path)
		}
		if scenario.coverage == contractCovered && scenario.mutation {
			if _, ok := mutationContractRegistry[path]; !ok {
				problems = append(problems, "covered mutation has no executable scenarios: "+path)
			}
		}
		if scenario.coverage == contractCovered && scenario.artifact {
			contract, ok := stage4ContractRegistry[path]
			if !ok || contract.success == nil || contract.failure == nil || contract.human == nil {
				problems = append(problems, "covered artifact has no executable scenarios: "+path)
			}
		}
	}
	sort.Strings(problems)
	if len(problems) != 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

func jsonLeafPaths(root *cobra.Command) map[string]bool {
	paths := make(map[string]bool)
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		children := cmd.Commands()
		if len(children) == 0 && (cmd.Run != nil || cmd.RunE != nil) &&
			clioutput.Class(cmd) == clioutput.ClassJSON {
			paths[cmd.CommandPath()] = true
		}
		for _, child := range children {
			visit(child)
		}
	}
	visit(root)
	return paths
}

func executeHumanContractCase(args []string) (string, string, error) {
	cmd := NewCommandForArgs(args)
	cmd.SetArgs(args)
	return captureProcessStreams(cmd.Execute)
}

func assertJSONDocument(t *testing.T, raw string) []byte {
	t.Helper()
	if strings.Contains(raw, "\x1b[") {
		t.Fatalf("JSON stream contains ANSI: %q", raw)
	}
	if err := validateJSONDocument([]byte(raw)); err != nil {
		t.Fatalf("invalid JSON stream framing: %v; raw=%q", err, raw)
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	var document json.RawMessage
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("decode JSON document: %v; raw=%q", err, raw)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Fatalf("stream does not end after one JSON document: %v; raw=%q", err, raw)
	}
	return document
}
