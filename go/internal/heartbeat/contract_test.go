package heartbeat

import "testing"

func TestCurrentContractContainsAllHeartbeatValues(t *testing.T) {
	contract := CurrentContract()
	if contract.Schema != "xp2p-heartbeat-contract" || contract.Version != ContractVersion {
		t.Fatalf("unexpected contract identity: %+v", contract)
	}
	assertValues(t, contract.Modes, ModeAuto, ModeRequired, ModeDisabled)
	assertValues(t, contract.Capabilities, CapabilityUnknown, CapabilityXP2PHeartbeat, CapabilityXP2PDiag)
	assertValues(t, contract.LegacyCapabilities, CapabilityDetected)
	assertValues(t, contract.Statuses, StatusProbing, StatusNotDetected, StatusHealthy, StatusUnhealthy, StatusDisabled)
	assertValues(t, contract.FailureStages, FailureStageMarker, FailureStageProbe, FailureStageReport, FailureStagePersistence)
	if contract.Thresholds.DiscoveryFailures != DiscoveryFailureThreshold ||
		contract.Thresholds.HealthFailures != HealthFailureThreshold {
		t.Fatalf("unexpected thresholds: %+v", contract.Thresholds)
	}
}

func assertValues[T comparable](t *testing.T, got []T, want ...T) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("values = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("values = %v, want %v", got, want)
		}
	}
}
