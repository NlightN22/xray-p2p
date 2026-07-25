package root

import "testing"

var stage6Paths = []string{
	"xp2p client dns-forward add",
	"xp2p client dns-forward list",
	"xp2p client dns-forward remove",
	"xp2p server dns-forward add",
	"xp2p server dns-forward list",
	"xp2p server dns-forward remove",
	"xp2p nat-redirect add",
	"xp2p nat-redirect list",
	"xp2p nat-redirect remove",
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
	baseline := buildLegacyPendingBaseline()
	for _, path := range stage6Paths {
		if baseline[path].coverage != contractStage6 {
			t.Errorf("stale stage 6 descriptor: %s", path)
		}
		if scenario := contractCaseRegistry[path]; scenario.coverage != contractCovered {
			t.Errorf("stage 6 leaf is not covered: %s", path)
		}
	}
}
