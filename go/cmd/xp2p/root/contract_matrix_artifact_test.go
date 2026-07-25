package root

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

type stage4Contract struct {
	success func(*testing.T, string)
	failure func(*testing.T, string)
	human   func(*testing.T, string)
}

var stage4ContractRegistry = buildStage4ContractRegistry()

func stage4ExpectedPaths() map[string]struct{} {
	return map[string]struct{}{
		"xp2p client debug bundle":       {},
		"xp2p client deploy":             {},
		"xp2p client export":             {},
		"xp2p client import":             {},
		"xp2p client install":            {},
		"xp2p server debug bundle":       {},
		"xp2p server export":             {},
		"xp2p server identity provision": {},
		"xp2p server import":             {},
		"xp2p server install":            {},
		"xp2p server user add":           {},
		"xp2p server user rotate":        {},
	}
}

func buildStage4ContractRegistry() map[string]stage4Contract {
	registry := map[string]stage4Contract{
		"xp2p client debug bundle":       archiveStage4Contract("client", "debug"),
		"xp2p client deploy":             clientDeployStage4Contract(),
		"xp2p client export":             archiveStage4Contract("client", "export"),
		"xp2p client import":             archiveStage4Contract("client", "import"),
		"xp2p client install":            clientInstallStage4Contract(),
		"xp2p server debug bundle":       archiveStage4Contract("server", "debug"),
		"xp2p server export":             archiveStage4Contract("server", "export"),
		"xp2p server identity provision": identityProvisionStage4Contract(),
		"xp2p server import":             archiveStage4Contract("server", "import"),
		"xp2p server install":            serverInstallStage4Contract(),
		"xp2p server user add":           userAddStage4Contract(),
		"xp2p server user rotate":        userRotateStage4Contract(),
	}
	return registry
}

func TestStage4LeavesCovered(t *testing.T) {
	if err := validateStage4Contracts(
		stage4ExpectedPaths(),
		contractCaseRegistry,
		stage4ContractRegistry,
	); err != nil {
		t.Fatal(err)
	}
}

func TestStage4GateRejectsMissingIncompleteAndStaleDescriptors(t *testing.T) {
	expected := map[string]struct{}{"xp2p client export": {}}
	covered := map[string]contractCase{
		"xp2p client export": {coverage: contractCovered, artifact: true},
	}
	complete := func(*testing.T, string) {}
	tests := []struct {
		name        string
		descriptors map[string]stage4Contract
		want        string
	}{
		{name: "missing", descriptors: map[string]stage4Contract{}, want: "missing descriptor"},
		{
			name: "incomplete",
			descriptors: map[string]stage4Contract{
				"xp2p client export": {success: complete, failure: complete},
			},
			want: "incomplete descriptor",
		},
		{
			name: "stale",
			descriptors: map[string]stage4Contract{
				"xp2p client export": {success: complete, failure: complete, human: complete},
				"xp2p stale":         {success: complete, failure: complete, human: complete},
			},
			want: "stale descriptor",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateStage4Contracts(expected, covered, test.descriptors)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got error %v, want %q", err, test.want)
			}
		})
	}
}

func TestStage4ContractCases(t *testing.T) {
	paths := make([]string, 0, len(stage4ContractRegistry))
	for path := range stage4ContractRegistry {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		path := path
		contract := stage4ContractRegistry[path]
		t.Run(path+"/success", func(t *testing.T) {
			contract.success(t, path)
		})
		t.Run(path+"/handler-error", func(t *testing.T) {
			contract.failure(t, path)
		})
		t.Run(path+"/human", func(t *testing.T) {
			contract.human(t, path)
		})
	}
}

func registerStage4ContractCase(
	registry map[string]contractCase,
	path string,
) {
	contract, ok := stage4ContractRegistry[path]
	if !ok || contract.success == nil || contract.failure == nil || contract.human == nil {
		panic(fmt.Sprintf("incomplete stage 4 contract: %s", path))
	}
	registry[path] = contractCase{coverage: contractCovered, artifact: true}
}

func validateStage4Contracts(
	expected map[string]struct{},
	registry map[string]contractCase,
	descriptors map[string]stage4Contract,
) error {
	var problems []string
	for path := range expected {
		scenario, ok := registry[path]
		if !ok || scenario.coverage != contractCovered || !scenario.artifact {
			problems = append(problems, "stage 4 leaf is not covered: "+path)
		}
		descriptor, ok := descriptors[path]
		if !ok {
			problems = append(problems, "missing descriptor: "+path)
		} else if descriptor.success == nil || descriptor.failure == nil || descriptor.human == nil {
			problems = append(problems, "incomplete descriptor: "+path)
		}
	}
	for path := range descriptors {
		if _, ok := expected[path]; !ok {
			problems = append(problems, "stale descriptor: "+path)
		}
	}
	sort.Strings(problems)
	if len(problems) != 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}
