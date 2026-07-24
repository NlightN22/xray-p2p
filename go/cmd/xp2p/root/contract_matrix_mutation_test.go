package root

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

type mutationFixture struct {
	args          []string
	failureArgs   []string
	sensitive     []string
	snapshot      func(*testing.T) any
	assertSuccess func(*testing.T, any, any)
}

type mutationContract struct {
	successFixture func(*testing.T) mutationFixture
	failureFixture func(*testing.T) mutationFixture
}

var mutationContractRegistry = buildMutationContractRegistry()

func buildMutationContractRegistry() map[string]mutationContract {
	registry := make(map[string]mutationContract)
	registerClientMutationContracts(registry)
	registerServerMutationContracts(registry)
	registerHAMutationContracts(registry)
	registerIdentityMutationContracts(registry)
	return registry
}

func TestStage3MutationLeavesCovered(t *testing.T) {
	baseline := buildLegacyPendingBaseline()
	covered := 0
	for path, legacy := range baseline {
		if legacy.coverage != contractStage3 {
			continue
		}
		covered++
		scenario, exists := contractCaseRegistry[path]
		if !exists || scenario.coverage != contractCovered || !scenario.mutation {
			t.Errorf("stage 3 mutation %s is not covered", path)
		}
		if _, exists := mutationContractRegistry[path]; !exists {
			t.Errorf("stage 3 mutation %s has no executable contract", path)
		}
	}
	if covered != 52 {
		t.Fatalf("stage 3 baseline has %d leaves, want 52", covered)
	}
	for path, scenario := range contractCaseRegistry {
		if scenario.coverage == contractStage3 {
			t.Errorf("stage 3 pending status remains: %s", path)
		}
	}
}

func TestStage3MutationContractCases(t *testing.T) {
	for path, scenario := range mutationContractRegistry {
		path, scenario := path, scenario
		t.Run(path+"/staged-success", func(t *testing.T) {
			fixture := scenario.successFixture(t)
			before := fixture.snapshot(t)
			execution := executeContractCase(fixture.args, false)
			assertMutationSuccess(t, path, execution, fixture.sensitive)
			after := fixture.snapshot(t)
			if reflect.DeepEqual(before, after) {
				t.Fatal("successful mutation did not change Desired state")
			}
			fixture.assertSuccess(t, before, after)
		})
		t.Run(path+"/handler-error-atomic", func(t *testing.T) {
			fixture := scenario.failureFixture(t)
			before := fixture.snapshot(t)
			execution := executeContractCase(fixture.failureArgs, false)
			assertMutationFailure(t, path, execution, fixture.sensitive)
			after := fixture.snapshot(t)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("failed mutation changed state:\nbefore=%#v\nafter=%#v", before, after)
			}
		})
		t.Run(path+"/human", func(t *testing.T) {
			fixture := scenario.successFixture(t)
			stdout, stderr, err := executeHumanContractCase(fixture.args)
			if err != nil {
				t.Fatalf("human execution failed: %v; stdout=%q stderr=%q", err, stdout, stderr)
			}
			if strings.Contains(stdout+stderr, "\x1b[") {
				t.Fatalf("human output contains ANSI: stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

func assertMutationSuccess(t *testing.T, path string, execution contractExecution, sensitive []string) {
	t.Helper()
	if execution.exitCode != 0 || execution.err != nil {
		t.Fatalf("exit=%d err=%v stderr=%q", execution.exitCode, execution.err, execution.stderr)
	}
	if execution.stderr != "" {
		t.Fatalf("stderr=%q", execution.stderr)
	}
	document := assertJSONDocument(t, execution.stdout)
	var envelope struct {
		SchemaVersion string         `json:"schema_version"`
		Command       string         `json:"command"`
		Result        mutationResult `json:"result"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	wantOperation := strings.TrimPrefix(path, "xp2p ")
	if envelope.SchemaVersion != clioutput.SchemaVersion || envelope.Command != path ||
		envelope.Result.Status != "completed" || envelope.Result.Operation != wantOperation ||
		strings.TrimSpace(envelope.Result.Entity) == "" {
		t.Fatalf("unexpected mutation envelope: %#v", envelope)
	}
	for _, forbidden := range append([]string{"\x1b[", "PRIVATE KEY"}, sensitive...) {
		if forbidden != "" && strings.Contains(strings.ToLower(execution.stdout), strings.ToLower(forbidden)) {
			t.Fatalf("mutation result leaked %q: %q", forbidden, execution.stdout)
		}
	}
}

func assertMutationFailure(t *testing.T, path string, execution contractExecution, sensitive []string) {
	t.Helper()
	if execution.exitCode == 0 || execution.err == nil {
		t.Fatalf("handler failure has exit=%d err=%v", execution.exitCode, execution.err)
	}
	if execution.stdout != "" {
		t.Fatalf("failure stdout=%q", execution.stdout)
	}
	document := assertJSONDocument(t, execution.stderr)
	var envelope clioutput.ErrorEnvelope
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != clioutput.SchemaVersion || envelope.Command != path ||
		envelope.Error.Code != "command_failed" {
		t.Fatalf("unexpected error envelope: %#v", envelope)
	}
	raw := strings.ToLower(execution.stderr)
	for _, forbidden := range append([]string{"\x1b[", "private key"}, sensitive...) {
		if forbidden != "" && strings.Contains(raw, strings.ToLower(forbidden)) {
			t.Fatalf("failure leaked %q: %q", forbidden, execution.stderr)
		}
	}
}

func mutationCase() contractCase {
	return contractCase{coverage: contractCovered, mutation: true}
}

func registerMutation(
	registry map[string]mutationContract,
	path string,
	successFixture func(*testing.T) mutationFixture,
	failureFixture func(*testing.T) mutationFixture,
) {
	if _, exists := registry[path]; exists {
		panic(fmt.Sprintf("duplicate mutation contract: %s", path))
	}
	registry[path] = mutationContract{
		successFixture: successFixture,
		failureFixture: failureFixture,
	}
}
