package redirect

import "testing"

func TestAccessPolicyNormalizesLegacyAndSelectors(t *testing.T) {
	legacy, err := (AccessPolicy{}).Normalized()
	if err != nil || legacy.Access != "all" {
		t.Fatalf("legacy = %#v, %v", legacy, err)
	}
	policy, err := (AccessPolicy{Users: []string{" Alice ", "alice"}}).Normalized()
	if err != nil || policy.Access != "restricted" || len(policy.Users) != 1 {
		t.Fatalf("policy = %#v, %v", policy, err)
	}
}

func TestRestrictedPolicyWithoutSelectorsFails(t *testing.T) {
	if _, err := (AccessPolicy{Access: "restricted"}).Normalized(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRestrictedRuleCompilesUsers(t *testing.T) {
	rules := BuildXrayRules("server", []Rule{{Domain: "example.test", OutboundTag: "reverse", AccessPolicy: AccessPolicy{Access: "restricted", Users: []string{"unknown"}}}})
	entry := rules[0].(map[string]any)
	users := entry["user"].([]string)
	if len(users) != 1 || users[0] != "unknown" {
		t.Fatalf("users = %#v", users)
	}
}
