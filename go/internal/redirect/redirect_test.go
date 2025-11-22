package redirect

import (
	"errors"
	"testing"
)

func TestResolveBinding(t *testing.T) {
	bindings := []Binding{
		{Tag: "alpha-tag", Host: "edge-a"},
		{Tag: "beta-tag", Host: "edge-b"},
	}

	binding, err := ResolveBinding("", "EDGE-A", bindings)
	if err != nil {
		t.Fatalf("ResolveBinding by host returned error: %v", err)
	}
	if binding.Tag != "alpha-tag" {
		t.Fatalf("unexpected tag %s", binding.Tag)
	}

	if _, err := ResolveBinding("alpha-tag", "beta", bindings); !errors.Is(err, ErrBindingHostNotFound) {
		t.Fatalf("expected host not found error, got %v", err)
	}

	binding, err = ResolveBinding("beta-tag", "", bindings)
	if err != nil {
		t.Fatalf("ResolveBinding by tag returned error: %v", err)
	}
	if binding.Host != "edge-b" {
		t.Fatalf("unexpected host %s", binding.Host)
	}

	if err := testBindingMismatch(bindings); err != ErrBindingTagMismatch {
		t.Fatalf("expected mismatch error, got %v", err)
	}
	if _, err := ResolveBinding("", "", bindings); !errors.Is(err, ErrBindingNotSpecified) {
		t.Fatalf("expected not specified error, got %v", err)
	}
	if _, err := ResolveBinding("unknown", "", bindings); !errors.Is(err, ErrBindingTagNotFound) {
		t.Fatalf("expected tag not found error, got %v", err)
	}
}

func testBindingMismatch(bindings []Binding) error {
	b, err := ResolveBinding("alpha-tag", "edge-b", bindings)
	if err == nil && b.Tag != "" {
		return ErrBindingTagMismatch
	}
	return err
}

func TestResolveRuleAndNormalize(t *testing.T) {
	target, err := ResolveRule(" 10.0.0.0/24 ", "")
	if err != nil {
		t.Fatalf("ResolveRule CIDR error: %v", err)
	}
	if target.Kind != KindCIDR || target.Value != "10.0.0.0/24" {
		t.Fatalf("unexpected target %+v", target)
	}

	target, err = ResolveRule("", "Example.COM")
	if err != nil {
		t.Fatalf("ResolveRule domain error: %v", err)
	}
	if target.Kind != KindDomain || target.Value != "example.com" {
		t.Fatalf("unexpected domain target %+v", target)
	}

	if _, err = ResolveRule("10.0.0.0/24", "example.com"); err == nil {
		t.Fatalf("expected error for both cidr and domain")
	}
	if _, err = NormalizeCIDR(""); err == nil {
		t.Fatalf("expected error for empty cidr")
	}
	if _, err = NormalizeCIDR("invalid"); err == nil {
		t.Fatalf("expected error for invalid cidr")
	}
	if _, err = NormalizeDomain("bad domain"); err == nil {
		t.Fatalf("expected error for invalid domain")
	}
}

func TestDescribeAndMatches(t *testing.T) {
	target := Target{Kind: KindDomain, Value: "example.com"}
	if desc := target.Describe(); desc != "domain example.com" {
		t.Fatalf("unexpected describe %s", desc)
	}
	rule := Rule{Domain: "Example.com", OutboundTag: "alpha"}
	if !target.Matches(rule) {
		t.Fatalf("expected target to match domain rule")
	}
	if desc := Describe(KindCIDR, "10.0.0.0/24"); desc != "CIDR 10.0.0.0/24" {
		t.Fatalf("Describe returned unexpected value %s", desc)
	}
}

func TestRuleKindAndValue(t *testing.T) {
	rule := Rule{Domain: "Example.com"}
	if rule.Kind() != KindDomain || rule.Value() != "example.com" {
		t.Fatalf("unexpected domain rule %v %s", rule.Kind(), rule.Value())
	}
	rule = Rule{CIDR: "10.0.0.0/24"}
	if rule.Kind() != KindCIDR || rule.Value() != "10.0.0.0/24" {
		t.Fatalf("unexpected CIDR rule %v %s", rule.Kind(), rule.Value())
	}
}

func TestAddRule(t *testing.T) {
	rules := []Rule{}
	rule := Rule{CIDR: "10.0.0.0/24", OutboundTag: "alpha"}
	updated, err := AddRule(rules, rule)
	if err != nil {
		t.Fatalf("AddRule error: %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("expected rule appended")
	}
	if _, err := AddRule(updated, rule); err == nil {
		t.Fatalf("expected duplicate error")
	}
	if _, err := AddRule(updated, Rule{CIDR: "", OutboundTag: "alpha"}); err == nil {
		t.Fatalf("expected missing value error")
	}
	if _, err := AddRule(updated, Rule{CIDR: "10.0.0.0/24"}); err == nil {
		t.Fatalf("expected missing tag error")
	}
}

func TestRemoveRule(t *testing.T) {
	rules := []Rule{
		{CIDR: "10.0.0.0/24", OutboundTag: "alpha"},
		{CIDR: "10.0.1.0/24", OutboundTag: "beta"},
	}
	target := Target{Kind: KindCIDR, Value: "10.0.0.0/24"}
	updated, removed := RemoveRule(rules, target, "")
	if !removed || len(updated) != 1 {
		t.Fatalf("RemoveRule should drop matching entry")
	}
	_, removed = RemoveRule(rules, target, "beta")
	if removed {
		t.Fatalf("RemoveRule should not remove when tag mismatches")
	}
	empty, removed := RemoveRule(nil, target, "")
	if removed || len(empty) != 0 {
		t.Fatalf("RemoveRule on nil slice should be a no-op")
	}
}
