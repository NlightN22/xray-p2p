package client

import "testing"

func TestWindowsDirectRulesHaveRuleTags(t *testing.T) {
	for _, raw := range windowsDirectRules() {
		rule, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("expected rule map, got %T", raw)
		}
		if tag, _ := rule["ruleTag"].(string); tag == "" {
			t.Fatalf("expected windows direct ruleTag, got %+v", rule)
		}
	}
}
