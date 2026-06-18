package runtimeapply

import "testing"

func TestClassifyXrayConfigDiffDetectsRoutingOnlyAddRemove(t *testing.T) {
	current := []byte(`{
		"log": {"loglevel": "warning"},
		"routing": {
			"domainStrategy": "AsIs",
			"rules": [
				{"type": "field", "ruleTag": "keep", "outboundTag": "direct"},
				{"type": "field", "ruleTag": "remove", "outboundTag": "proxy-a"}
			]
		}
	}`)
	candidate := []byte(`{
		"log": {"loglevel": "warning"},
		"routing": {
			"domainStrategy": "AsIs",
			"rules": [
				{"type": "field", "ruleTag": "keep", "outboundTag": "direct"},
				{"type": "field", "ruleTag": "add", "outboundTag": "proxy-b"}
			]
		}
	}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffRoutingOnly {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffRoutingOnly, diff)
	}
	if len(diff.AddedRules) != 1 || diff.AddedRules[0].RuleTag != "add" {
		t.Fatalf("unexpected added rules: %+v", diff.AddedRules)
	}
	if len(diff.RemovedRuleTag) != 1 || diff.RemovedRuleTag[0] != "remove" {
		t.Fatalf("unexpected removed rules: %+v", diff.RemovedRuleTag)
	}
}

func TestClassifyXrayConfigDiffRejectsGlobalChange(t *testing.T) {
	current := []byte(`{"log":{"loglevel":"warning"},"routing":{"rules":[{"ruleTag":"a"}]}}`)
	candidate := []byte(`{"log":{"loglevel":"debug"},"routing":{"rules":[{"ruleTag":"a"}]}}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffUnsupported {
		t.Fatalf("kind = %s, want unsupported", diff.Kind)
	}
}

func TestClassifyXrayConfigDiffRejectsUntaggedRules(t *testing.T) {
	current := []byte(`{"routing":{"rules":[{"outboundTag":"direct"}]}}`)
	candidate := []byte(`{"routing":{"rules":[{"outboundTag":"direct"},{"ruleTag":"add"}]}}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffUnsupported {
		t.Fatalf("kind = %s, want unsupported", diff.Kind)
	}
}

func TestClassifyXrayConfigDiffRejectsTaggedRuleMutation(t *testing.T) {
	current := []byte(`{"routing":{"rules":[{"ruleTag":"a","outboundTag":"direct"}]}}`)
	candidate := []byte(`{"routing":{"rules":[{"ruleTag":"a","outboundTag":"proxy"}]}}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffUnsupported {
		t.Fatalf("kind = %s, want unsupported", diff.Kind)
	}
}

func TestClassifyXrayConfigDiffNoop(t *testing.T) {
	current := []byte(`{"routing":{"rules":[{"ruleTag":"a","outboundTag":"direct"}]}}`)

	diff, err := ClassifyXrayConfigDiff(current, current)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffNoop {
		t.Fatalf("kind = %s, want noop", diff.Kind)
	}
}
