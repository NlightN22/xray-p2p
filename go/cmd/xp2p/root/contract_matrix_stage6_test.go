package root

import (
	"sort"
	"testing"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

var stage6Paths = stage6PlatformPaths()

func stage6PlatformPaths() []string {
	paths := make([]string, 0, len(platformSpecificOutputContracts))
	for path := range platformSpecificOutputContracts {
		if contract, ok := outputContractInventory[path]; ok && contract.class == clioutput.ClassJSON {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func registerStage6ContractCases(registry map[string]contractCase) {
	for _, path := range stage6Paths {
		scenario := registry[path]
		scenario.coverage = contractCovered
		scenario.platformCase = true
		scenario.platform = "linux"
		scenario.credentialPolicy = "platform network and firewall results omit credentials"
		registry[path] = scenario
	}
	registerStage6PlatformContractCases(registry)
}

func TestStage6LeavesCovered(t *testing.T) {
	for _, path := range stage6Paths {
		if scenario := contractCaseRegistry[path]; scenario.coverage != contractCovered {
			t.Errorf("stage 6 leaf is not covered: %s", path)
		}
	}
}

func TestStage6GateDerivesLeavesFromPlatformClassification(t *testing.T) {
	if len(stage6Paths) != len(platformSpecificOutputContracts) {
		t.Fatalf("stage 6 gate omitted a platform-specific output contract: paths=%v classification=%v",
			stage6Paths, platformSpecificOutputContracts)
	}
}
