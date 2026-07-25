package root

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/pelletier/go-toml"
)

type mutationFixture struct {
	args           []string
	failureArgs    []string
	sensitive      []string
	expectedEntity string
	snapshot       func(*testing.T) any
	assertSuccess  func(*testing.T, any, any)
}

type mutationContract struct {
	successFixture func(*testing.T) mutationFixture
	failureFixture func(*testing.T) mutationFixture
}

type modeMutationSnapshot struct {
	desired     string
	applyExists bool
	apply       string
}

type persistedCredentialDocument struct {
	Client struct {
		Endpoints []struct {
			Password string `toml:"password"`
		} `toml:"endpoints"`
	} `toml:"client"`
	Server struct {
		TrojanUsers []struct {
			Password string `toml:"password"`
		} `toml:"trojan_users"`
	} `toml:"server"`
}

func readPersistedCredentials(t *testing.T, path string) persistedCredentialDocument {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document persistedCredentialDocument
	if err := toml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func snapshotModeMutation(t *testing.T, desiredPath string) modeMutationSnapshot {
	t.Helper()
	desired, err := os.ReadFile(desiredPath)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := modeMutationSnapshot{desired: string(desired)}
	request, err := os.ReadFile(config.ApplyRequestPath())
	if err == nil {
		snapshot.applyExists = true
		snapshot.apply = string(request)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return snapshot
}

var mutationContractRegistry = buildMutationContractRegistry()

var stage3PolymorphicMutationPaths = map[string]struct{}{
	"xp2p client mode": {},
	"xp2p server mode": {},
}

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
	expected := make(map[string]struct{})
	for path, legacy := range baseline {
		if legacy.coverage != contractStage3 {
			continue
		}
		expected[path] = struct{}{}
		scenario, exists := contractCaseRegistry[path]
		if !exists || scenario.coverage != contractCovered || !scenario.mutation {
			t.Errorf("stage 3 mutation %s is not covered", path)
		}
		if _, exists := mutationContractRegistry[path]; !exists {
			t.Errorf("stage 3 mutation %s has no executable contract", path)
		}
		if contract := outputContractInventory[path]; contract.successResult != nil {
			t.Errorf("stage 3 mutation %s uses a root-level synthetic result", path)
		}
	}
	for path := range stage3PolymorphicMutationPaths {
		expected[path] = struct{}{}
		scenario, exists := mutationContractRegistry[path]
		if !exists || scenario.successFixture == nil || scenario.failureFixture == nil {
			t.Errorf("polymorphic mode mutation %s has no executable contract", path)
		}
	}
	for path := range expected {
		if _, exists := mutationContractRegistry[path]; !exists {
			t.Errorf("expected stage 3 mutation %s is missing from the registry", path)
		}
	}
	for path := range mutationContractRegistry {
		if _, exists := expected[path]; !exists {
			t.Errorf("mutation registry contains undeclared stage 3 variant %s", path)
		}
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
			assertMutationSuccess(t, path, execution, fixture.sensitive, fixture.expectedEntity)
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
			assertMutationHumanBaseline(t, path, stdout, stderr)
		})
	}
}

func assertMutationHumanBaseline(t *testing.T, path, stdout, stderr string) {
	t.Helper()
	normalized := normalizeHumanOutput(stdout) + "\x00" + normalizeHumanOutput(stderr)
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(normalized)))
	expected, exists := mutationHumanBaselineDigests[path]
	if platformExpected := mutationHumanPlatformBaselineDigests[path][runtime.GOOS]; platformExpected != "" {
		expected = platformExpected
		exists = true
	}
	if !exists {
		t.Fatalf("missing mutation human baseline for %s: digest=%s normalized=%q", path, digest, normalized)
	}
	if digest != expected {
		t.Fatalf(
			"mutation human baseline changed for %s: got=%s want=%s normalized=%q",
			path,
			digest,
			expected,
			normalized,
		)
	}
}

func assertMutationSuccess(
	t *testing.T,
	path string,
	execution contractExecution,
	sensitive []string,
	expectedEntity string,
) {
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
		envelope.Result.Entity != expectedEntity {
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
		successFixture: withMutationEntity(path, successFixture),
		failureFixture: withMutationEntity(path, failureFixture),
	}
}

func withMutationEntity(
	path string,
	factory func(*testing.T) mutationFixture,
) func(*testing.T) mutationFixture {
	return func(t *testing.T) mutationFixture {
		fixture := factory(t)
		fixture.expectedEntity = expectedMutationEntities[path]
		if fixture.expectedEntity == "" {
			t.Fatalf("mutation %s has no expected entity", path)
		}
		return fixture
	}
}
