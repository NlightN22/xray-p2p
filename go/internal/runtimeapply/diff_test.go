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

func TestClassifyXrayConfigDiffDetectsInboundOnlyAddRemove(t *testing.T) {
	current := []byte(`{
		"log": {"loglevel": "warning"},
		"inbounds": [
			{"tag": "keep", "protocol": "socks"},
			{"tag": "old", "protocol": "dokodemo-door"}
		],
		"routing": {"rules": [{"ruleTag": "route-a"}]}
	}`)
	candidate := []byte(`{
		"log": {"loglevel": "warning"},
		"inbounds": [
			{"tag": "keep", "protocol": "socks"},
			{"tag": "new", "protocol": "dokodemo-door"}
		],
		"routing": {"rules": [{"ruleTag": "route-a"}]}
	}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffInboundOnly {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffInboundOnly, diff)
	}
	if len(diff.AddedInbounds) != 1 || diff.AddedInbounds[0].Tag != "new" {
		t.Fatalf("unexpected added inbounds: %+v", diff.AddedInbounds)
	}
	if len(diff.RemovedInboundTags) != 1 || diff.RemovedInboundTags[0] != "old" {
		t.Fatalf("unexpected removed inbounds: %+v", diff.RemovedInboundTags)
	}
}

func TestClassifyXrayConfigDiffRejectsInboundGlobalChange(t *testing.T) {
	current := []byte(`{"log":{"loglevel":"warning"},"inbounds":[{"tag":"a"}]}`)
	candidate := []byte(`{"log":{"loglevel":"debug"},"inbounds":[{"tag":"a"},{"tag":"b"}]}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffUnsupported {
		t.Fatalf("kind = %s, want unsupported", diff.Kind)
	}
}

func TestClassifyXrayConfigDiffRejectsTaggedInboundMutation(t *testing.T) {
	current := []byte(`{"inbounds":[{"tag":"a","protocol":"socks"}]}`)
	candidate := []byte(`{"inbounds":[{"tag":"a","protocol":"dokodemo-door"}]}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffUnsupported {
		t.Fatalf("kind = %s, want unsupported", diff.Kind)
	}
}

func TestClassifyXrayConfigDiffDetectsOutboundOnlyAddRemove(t *testing.T) {
	current := []byte(`{
		"log": {"loglevel": "warning"},
		"outbounds": [
			{"tag": "keep", "protocol": "freedom"},
			{"tag": "old", "protocol": "trojan"}
		],
		"routing": {"rules": [{"ruleTag": "route-a"}]}
	}`)
	candidate := []byte(`{
		"log": {"loglevel": "warning"},
		"outbounds": [
			{"tag": "keep", "protocol": "freedom"},
			{"tag": "new", "protocol": "trojan"}
		],
		"routing": {"rules": [{"ruleTag": "route-a"}]}
	}`)

	diff, err := ClassifyXrayConfigDiff(current, candidate)
	if err != nil {
		t.Fatalf("ClassifyXrayConfigDiff: %v", err)
	}
	if diff.Kind != DiffOutboundOnly {
		t.Fatalf("kind = %s, want %s: %+v", diff.Kind, DiffOutboundOnly, diff)
	}
	if len(diff.AddedOutbounds) != 1 || diff.AddedOutbounds[0].Tag != "new" {
		t.Fatalf("unexpected added outbounds: %+v", diff.AddedOutbounds)
	}
	if len(diff.RemovedOutboundTags) != 1 || diff.RemovedOutboundTags[0] != "old" {
		t.Fatalf("unexpected removed outbounds: %+v", diff.RemovedOutboundTags)
	}
}

func TestClassifyXrayConfigDiffRejectsTaggedOutboundMutation(t *testing.T) {
	current := []byte(`{"outbounds":[{"tag":"a","protocol":"freedom"}]}`)
	candidate := []byte(`{"outbounds":[{"tag":"a","protocol":"trojan"}]}`)

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
