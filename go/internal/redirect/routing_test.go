package redirect

import "testing"

func TestBuildXrayRulesSortsManagedRedirects(t *testing.T) {
	rules := []Rule{
		{CIDR: "10.0.0.0/8", OutboundTag: "proxy-b"},
		{CIDR: "10.20.0.0/16", OutboundTag: "proxy-a"},
		{Domain: "example.com", OutboundTag: "proxy-b"},
		{Domain: "full:reverse.internal", OutboundTag: "proxy-c"},
		{CIDR: "10.20.30.40/32", OutboundTag: "proxy-a"},
	}

	got := BuildXrayRules("client", rules)
	if len(got) != 5 {
		t.Fatalf("rules = %d, want 5", len(got))
	}

	want := []struct {
		field string
		value string
	}{
		{field: "domains", value: "domain:example.com"},
		{field: "domains", value: "full:reverse.internal"},
		{field: "ip", value: "10.20.30.40/32"},
		{field: "ip", value: "10.20.0.0/16"},
		{field: "ip", value: "10.0.0.0/8"},
	}
	for idx, item := range want {
		rule, ok := got[idx].(map[string]any)
		if !ok {
			t.Fatalf("rule %d is %T, want map", idx, got[idx])
		}
		values, _ := rule[item.field].([]string)
		if len(values) != 1 || values[0] != item.value {
			t.Fatalf("rule %d %s = %v, want %s", idx, item.field, values, item.value)
		}
	}
}
