package root

import (
	"reflect"
	"sort"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/runtimeboundary"
)

var nonRuntimeMutationReasons = map[string]string{
	"xp2p client subscription add":           "registering a subscription source does not change active Xray resources",
	"xp2p client mode":                       "mode changes require service-layer TUN, route, DNS, and firewall handling",
	"xp2p server ha channel create":          "channel generation state does not change the active Xray resource set",
	"xp2p server ha channel disable":         "channel generation state does not change the active Xray resource set",
	"xp2p server ha channel finalize":        "channel generation state does not change the active Xray resource set",
	"xp2p server ha channel rebind":          "channel generation state does not change the active Xray resource set",
	"xp2p server ha channel rebind-endpoint": "channel generation state does not change the active Xray resource set",
	"xp2p server ha group remove":            "group generation state does not change the active Xray resource set",
	"xp2p server ha group update":            "group generation state does not change the active Xray resource set",
	"xp2p server ha group create":            "creating generation state does not change the active Xray resource set",
	"xp2p server ha member add":              "member generation state does not change the active Xray resource set",
	"xp2p server ha member remove":           "member generation state does not change the active Xray resource set",
	"xp2p server ha member reprioritize":     "member generation state does not change the active Xray resource set",
	"xp2p server ha peer add":                "peer transport trust is control-plane state, not an Xray runtime resource",
	"xp2p server ha peer remove":             "peer transport trust is control-plane state, not an Xray runtime resource",
	"xp2p server ha peer self":               "the local peer ID is control-plane state, not an Xray runtime resource",
	"xp2p server ha sync":                    "sync persists generated control-plane state without changing active Xray resources",
	"xp2p server identity detach":            "provider selection is identity control state; sync applies runtime resources",
	"xp2p server identity select":            "provider selection is identity control state; sync applies runtime resources",
	"xp2p server mode":                       "mode changes require service-layer TUN, route, DNS, and firewall handling",
}

func TestStage3RuntimeApplyContractCases(t *testing.T) {
	paths := make([]string, 0, len(mutationContractRegistry))
	for path := range mutationContractRegistry {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		path := path
		scenario := mutationContractRegistry[path]
		if reason := nonRuntimeMutationReasons[path]; reason != "" {
			t.Run(path+"/not-api-capable", func(t *testing.T) {
				if reason == "" {
					t.Fatal("non-runtime mutation requires an explicit reason")
				}
			})
			continue
		}
		t.Run(path+"/runtime-success", func(t *testing.T) {
			fixture := scenario.successFixture(t)
			role := mutationRole(path)
			recorder := newRuntimeAPIRecorder(t, path, role, runtimeAPISuccess)
			restore := runtimeboundary.SetForTesting(recorder.boundary())
			t.Cleanup(restore)
			beforeDomain := fixture.snapshot(t)

			execution := executeContractCase(fixture.args, false)
			assertMutationSuccess(t, path, execution, fixture.sensitive, fixture.expectedEntity)
			afterDomain := fixture.snapshot(t)
			if reflect.DeepEqual(beforeDomain, afterDomain) {
				t.Fatal("runtime success did not commit Desired/control state")
			}
			recorder.assertProductionFlow(t, role, true)
		})
		t.Run(path+"/runtime-verification-failure-atomic", func(t *testing.T) {
			fixture := scenario.successFixture(t)
			role := mutationRole(path)
			recorder := newRuntimeAPIRecorder(t, path, role, runtimeAPIFailVerification)
			before := snapshotRoleMutationState(t, role)
			restore := runtimeboundary.SetForTesting(recorder.boundary())
			t.Cleanup(restore)

			execution := executeContractCase(fixture.args, false)
			assertMutationFailure(t, path, execution, fixture.sensitive)
			after := snapshotRoleMutationState(t, role)
			assertRuntimeFailureState(t, before, after)
			recorder.assertProductionFlow(t, role, false)
		})
		t.Run(path+"/desired-persistence-failure-rollback", func(t *testing.T) {
			fixture := scenario.successFixture(t)
			role := mutationRole(path)
			recorder := newRuntimeAPIRecorder(t, path, role, runtimeAPIFailDesiredCommit)
			before := snapshotRoleMutationState(t, role)
			restore := runtimeboundary.SetForTesting(recorder.boundary())
			t.Cleanup(restore)

			execution := executeContractCase(fixture.args, false)
			recorder.restoreDesired(t)
			assertMutationFailure(t, path, execution, fixture.sensitive)
			after := snapshotRoleMutationState(t, role)
			assertRuntimeFailureState(t, before, after)
			recorder.assertProductionFlow(t, role, false)
		})
	}
}

func mutationRole(path string) string {
	if len(path) >= len("xp2p client ") && path[:len("xp2p client ")] == "xp2p client " {
		return apply.RoleClient
	}
	return apply.RoleServer
}
