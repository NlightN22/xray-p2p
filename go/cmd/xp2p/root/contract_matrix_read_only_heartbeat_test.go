package root

import (
	"strings"
	"testing"
)

func heartbeatContractCase() contractCase {
	args := []string{"heartbeat", "contract"}
	return contractCase{
		coverage:         contractCovered,
		success:          args,
		empty:            args,
		failure:          append(args, "unexpected"),
		failureCode:      "invalid_argument",
		setup:            func(*testing.T, string) {},
		assertResult:     assertHeartbeatContractResult,
		assertEmpty:      assertHeartbeatContractResult,
		emptyResult:      "the immutable protocol contract has no empty state; required enum arrays remain non-nil",
		credentialPolicy: "protocol metadata contains no credentials",
		edgeCases:        []string{"number", "ordered enum arrays", "non-empty invariant", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{`"schema": "xp2p-heartbeat-contract"`, `"version": 1`, `"modes"`, `"failure_stages"`, `"thresholds"`} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human heartbeat contract is missing %q: output=%q diagnostics=%q", expected, output, diagnostics)
				}
			}
		},
	}
}

func assertHeartbeatContractResult(t *testing.T, result map[string]any) {
	t.Helper()
	if result["schema"] != "xp2p-heartbeat-contract" || result["version"] != float64(1) {
		t.Fatalf("heartbeat contract header changed: %#v", result)
	}
	expected := map[string]int{
		"modes": 3, "capabilities": 3, "legacy_capabilities": 1,
		"checks": 4, "statuses": 5, "failure_stages": 4,
	}
	for key, count := range expected {
		items, ok := result[key].([]any)
		if !ok || items == nil || len(items) != count {
			t.Fatalf("%s=%#v, want non-nil array of %d", key, result[key], count)
		}
	}
	thresholds, ok := result["thresholds"].(map[string]any)
	if !ok || thresholds["discovery_failures"] != float64(3) ||
		thresholds["health_failures"] != float64(3) {
		t.Fatalf("heartbeat thresholds changed: %#v", result["thresholds"])
	}
}
