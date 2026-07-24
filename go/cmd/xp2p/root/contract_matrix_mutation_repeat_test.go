package root

import (
	"reflect"
	"sort"
	"testing"
)

type mutationRepeatPolicy struct {
	success      bool
	stateChanges bool
	reason       string
}

var mutationRepeatPolicies = buildMutationRepeatPolicies()

func TestStage3MutationRepeatContractCases(t *testing.T) {
	paths := make([]string, 0, len(mutationContractRegistry))
	for path := range mutationContractRegistry {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		path := path
		scenario := mutationContractRegistry[path]
		policy, exists := mutationRepeatPolicies[path]
		if !exists || policy.reason == "" {
			t.Errorf("%s has no documented repeat policy", path)
			continue
		}
		t.Run(path, func(t *testing.T) {
			fixture := scenario.successFixture(t)
			first := executeContractCase(fixture.args, false)
			assertMutationSuccess(t, path, first, fixture.sensitive, fixture.expectedEntity)
			beforeRepeat := fixture.snapshot(t)

			repeated := executeContractCase(fixture.args, false)
			if policy.success {
				assertMutationSuccess(t, path, repeated, fixture.sensitive, fixture.expectedEntity)
			} else {
				assertMutationFailure(t, path, repeated, fixture.sensitive)
			}
			afterRepeat := fixture.snapshot(t)
			changed := !reflect.DeepEqual(beforeRepeat, afterRepeat)
			if changed != policy.stateChanges {
				t.Fatalf("repeat state change=%t, want %t (%s)", changed, policy.stateChanges, policy.reason)
			}
		})
	}
}

func buildMutationRepeatPolicies() map[string]mutationRepeatPolicy {
	unchanged := func(reason string) mutationRepeatPolicy {
		return mutationRepeatPolicy{success: true, reason: reason}
	}
	rejected := func(reason string) mutationRepeatPolicy {
		return mutationRepeatPolicy{reason: reason}
	}
	advances := func(reason string) mutationRepeatPolicy {
		return mutationRepeatPolicy{success: true, stateChanges: true, reason: reason}
	}
	return map[string]mutationRepeatPolicy{
		"xp2p client disable":                      unchanged("setting the same endpoint state is idempotent"),
		"xp2p client enable":                       unchanged("setting the same endpoint state is idempotent"),
		"xp2p client forward add":                  rejected("a duplicate listen port is rejected"),
		"xp2p client forward remove":               rejected("removing a missing forward is rejected without --ignore-missing"),
		"xp2p client redirect add":                 unchanged("adding the same normalized redirect is idempotent"),
		"xp2p client redirect disable":             rejected("an already disabled redirect is not an enabled match"),
		"xp2p client redirect enable":              rejected("an already enabled redirect is not a disabled match"),
		"xp2p client redirect remove":              rejected("removing a missing redirect is rejected"),
		"xp2p client reverse disable":              unchanged("setting the same reverse state is idempotent"),
		"xp2p client reverse enable":               unchanged("setting the same reverse state is idempotent"),
		"xp2p client subscription add":             rejected("a duplicate subscription ID is rejected"),
		"xp2p client subscription refresh":         unchanged("refreshing an unchanged source is idempotent"),
		"xp2p client subscription remove":          rejected("removing a missing subscription is rejected"),
		"xp2p client update":                       unchanged("reapplying identical endpoint values is idempotent"),
		"xp2p server forward add":                  rejected("a duplicate listen port is rejected"),
		"xp2p server forward remove":               rejected("removing a missing forward is rejected without --ignore-missing"),
		"xp2p server ha channel create":            rejected("a duplicate channel ID is rejected"),
		"xp2p server ha channel disable":           advances("HA mutations commit a new generation"),
		"xp2p server ha channel finalize":          rejected("a finalized channel no longer exists"),
		"xp2p server ha channel rebind":            advances("HA mutations commit a new generation"),
		"xp2p server ha channel rebind-endpoint":   advances("HA mutations commit a new generation"),
		"xp2p server ha group create":              rejected("an initialized HA group cannot be created again"),
		"xp2p server ha group remove":              advances("HA removal is represented by a new tombstone generation"),
		"xp2p server ha group update":              advances("HA mutations commit a new generation"),
		"xp2p server ha member add":                rejected("a duplicate member ID is rejected"),
		"xp2p server ha member remove":             advances("repeated tombstoning commits an auditable HA generation"),
		"xp2p server ha member reprioritize":       advances("HA mutations commit a new generation"),
		"xp2p server ha peer add":                  unchanged("peer upsert with identical values is idempotent"),
		"xp2p server ha peer remove":               unchanged("peer removal is idempotent"),
		"xp2p server ha peer self":                 unchanged("setting the same local peer ID is idempotent"),
		"xp2p server ha redirect add":              rejected("a duplicate HA redirect is rejected"),
		"xp2p server ha redirect remove":           rejected("removing a missing HA redirect is rejected"),
		"xp2p server ha sync":                      advances("each synchronization commits a new HA generation"),
		"xp2p server identity detach":              unchanged("detaching an already detached provider is idempotent"),
		"xp2p server identity select":              unchanged("selecting the same provider is idempotent"),
		"xp2p server identity sync":                advances("each successful sync records a new identity generation"),
		"xp2p server redirect access add-group":    unchanged("adding an existing group is idempotent"),
		"xp2p server redirect access add-user":     unchanged("adding an existing user is idempotent"),
		"xp2p server redirect access clear":        unchanged("clearing an already clear policy is idempotent"),
		"xp2p server redirect access remove-group": unchanged("removing a missing group is idempotent"),
		"xp2p server redirect access remove-user":  unchanged("removing a missing user is idempotent"),
		"xp2p server redirect access set":          unchanged("setting the same normalized policy is idempotent"),
		"xp2p server redirect add":                 unchanged("adding the same normalized redirect is idempotent"),
		"xp2p server redirect disable":             rejected("an already disabled redirect is not an enabled match"),
		"xp2p server redirect enable":              rejected("an already enabled redirect is not a disabled match"),
		"xp2p server redirect remove":              rejected("removing a missing redirect is rejected"),
		"xp2p server reverse disable":              unchanged("setting the same reverse state is idempotent"),
		"xp2p server reverse enable":               unchanged("setting the same reverse state is idempotent"),
		"xp2p server user disable":                 unchanged("setting the same user state is idempotent"),
		"xp2p server user enable":                  unchanged("setting the same user state is idempotent"),
		"xp2p server user remove":                  unchanged("user removal is intentionally idempotent"),
		"xp2p server user update":                  rejected("the original user ID no longer exists after rename"),
	}
}
