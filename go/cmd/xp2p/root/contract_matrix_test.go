package root

import (
	"bytes"
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
	if got := pendingBaselineDigest(buildLegacyPendingBaseline()); got != legacyPendingBaselineDigest {
		t.Fatalf("legacy pending baseline changed: got %s want %s", got, legacyPendingBaselineDigest)
	}
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
	if err := validatePendingCases(contractCaseRegistry); err != nil {
		t.Fatal(err)
	}
}

func TestContractCaseRegistryDetectsMissingAndStaleCases(t *testing.T) {
	cases := map[string]contractCase{"xp2p client list": {coverage: contractStage2}}
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
			"xp2p client list": {coverage: contractStage2},
			"xp2p stale":       {coverage: contractStage2},
		},
	); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale case was not detected: %v", err)
	}
	if err := validatePendingCases(
		map[string]contractCase{"xp2p client new": {coverage: contractStage2}},
	); err == nil || !strings.Contains(err.Error(), "new command cannot be pending") {
		t.Fatalf("new pending case was not rejected: %v", err)
	}
	changedBaseline := buildLegacyPendingBaseline()
	changedBaseline["xp2p client new"] = contractCase{coverage: contractStage2}
	if got := pendingBaselineDigest(changedBaseline); got == legacyPendingBaselineDigest {
		t.Fatal("baseline digest did not detect a new pending command")
	}
}

func TestCoveredContractCases(t *testing.T) {
	for path, scenario := range contractCaseRegistry {
		if scenario.coverage != contractCovered {
			continue
		}
		t.Run(path+"/success", func(t *testing.T) {
			scenario.setup(t, "success")
			stdout, stderr, err := executeContractCase(scenario.success)
			if err != nil {
				t.Fatalf("execute: %v; stderr=%q", err, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr=%q", stderr)
			}
			document := assertJSONDocument(t, stdout)
			if stderr != "" {
				t.Fatalf("stderr=%q", stderr)
			}
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
		})
		t.Run(path+"/empty", func(t *testing.T) {
			scenario.setup(t, "empty")
			stdout, stderr, err := executeContractCase(scenario.empty)
			if err != nil {
				t.Fatalf("execute: %v; stderr=%q", err, stderr)
			}
			document := assertJSONDocument(t, stdout)
			var envelope struct {
				Result map[string]any `json:"result"`
			}
			if decodeErr := json.Unmarshal(document, &envelope); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			scenario.assertEmpty(t, envelope.Result)
		})
		t.Run(path+"/error", func(t *testing.T) {
			scenario.setup(t, "error")
			stdout, stderr, err := executeContractCase(scenario.failure)
			if err == nil {
				t.Fatal("expected handler error")
			}
			var exitCoder interface{ ExitCode() int }
			if errors.As(err, &exitCoder) && exitCoder.ExitCode() == 0 {
				t.Fatalf("handler error has a zero exit code: %T %v", err, err)
			}
			if stdout != "" {
				t.Fatalf("stdout=%q", stdout)
			}
			document := assertJSONDocument(t, stderr)
			var envelope clioutput.ErrorEnvelope
			if decodeErr := json.Unmarshal(document, &envelope); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if envelope.SchemaVersion != clioutput.SchemaVersion ||
				envelope.Command != path || envelope.Error.Code != "command_failed" {
				t.Fatalf("unexpected error envelope: %#v", envelope)
			}
			if strings.Contains(stderr, "\x1b[") {
				t.Fatalf("stderr leaked diagnostics: %q", stderr)
			}
		})
		t.Run(path+"/human", func(t *testing.T) {
			scenario.setup(t, "success")
			stdout, stderr, err := executeHumanContractCase(scenario.human)
			if err != nil {
				t.Fatalf("execute: %v; stderr=%q", err, stderr)
			}
			scenario.assertHuman(t, stdout, stderr)
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
		switch scenario.coverage {
		case contractCovered, contractStage2, contractStage3, contractStage4,
			contractStage5, contractStage6:
		default:
			problems = append(problems, "invalid coverage status: "+path)
		}
		if scenario.coverage == contractCovered &&
			(len(scenario.success) == 0 || len(scenario.empty) == 0 ||
				len(scenario.failure) == 0 ||
				scenario.setup == nil || scenario.assertResult == nil ||
				scenario.assertEmpty == nil ||
				scenario.emptyResult == "" || scenario.credentialPolicy == "" ||
				len(scenario.edgeCases) == 0 || scenario.platform == "" ||
				len(scenario.human) == 0 || scenario.assertHuman == nil) {
			problems = append(problems, "covered case has incomplete scenarios: "+path)
		}
	}
	sort.Strings(problems)
	if len(problems) != 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

func validatePendingCases(registry map[string]contractCase) error {
	baseline := buildLegacyPendingBaseline()
	var problems []string
	for path, scenario := range registry {
		if scenario.coverage == contractCovered {
			continue
		}
		baselineCase, existed := baseline[path]
		if !existed {
			problems = append(problems, "new command cannot be pending: "+path)
		} else if baselineCase.coverage != scenario.coverage {
			problems = append(problems, "pending stage changed: "+path)
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

func executeContractCase(args []string) (string, string, error) {
	allArgs := append([]string{"--json"}, args...)
	cmd := NewCommandForArgs(allArgs)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(allArgs)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
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
