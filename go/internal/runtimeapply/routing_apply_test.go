package runtimeapply

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestApplyRoutingDiffSequencesRemoveBeforeAdd(t *testing.T) {
	applier := &recordingRoutingApplier{}
	diff := Diff{
		Kind: DiffRoutingOnly,
		RemovedRules: []RoutingRuleChange{
			{RuleTag: "old", Rule: map[string]any{"ruleTag": "old"}},
		},
		AddedRules: []RoutingRuleChange{
			{RuleTag: "new", Rule: map[string]any{"ruleTag": "new"}},
		},
	}

	if err := ApplyRoutingDiff(context.Background(), applier, diff); err != nil {
		t.Fatalf("ApplyRoutingDiff: %v", err)
	}
	want := []string{"remove:old", "add:new"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

func TestApplyRoutingDiffRollsBackAddedRules(t *testing.T) {
	applier := &recordingRoutingApplier{failAddTag: "bad"}
	diff := Diff{
		Kind: DiffRoutingOnly,
		AddedRules: []RoutingRuleChange{
			{RuleTag: "good", Rule: map[string]any{"ruleTag": "good"}},
			{RuleTag: "bad", Rule: map[string]any{"ruleTag": "bad"}},
		},
	}

	err := ApplyRoutingDiff(context.Background(), applier, diff)
	if err == nil {
		t.Fatal("expected error")
	}
	want := []string{"add:good", "add:bad", "remove:good"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

func TestApplyRoutingDiffRestoresRemovedRules(t *testing.T) {
	applier := &recordingRoutingApplier{failRemoveTag: "bad"}
	diff := Diff{
		Kind: DiffRoutingOnly,
		RemovedRules: []RoutingRuleChange{
			{RuleTag: "good", Rule: map[string]any{"ruleTag": "good"}},
			{RuleTag: "bad", Rule: map[string]any{"ruleTag": "bad"}},
		},
	}

	err := ApplyRoutingDiff(context.Background(), applier, diff)
	if err == nil {
		t.Fatal("expected error")
	}
	want := []string{"remove:good", "remove:bad", "add:good"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

type recordingRoutingApplier struct {
	calls         []string
	failAddTag    string
	failRemoveTag string
}

func (a *recordingRoutingApplier) AddRule(_ context.Context, rule map[string]any) error {
	tag, _ := rule["ruleTag"].(string)
	a.calls = append(a.calls, "add:"+tag)
	if tag == a.failAddTag {
		return errors.New("add failed")
	}
	return nil
}

func (a *recordingRoutingApplier) RemoveRule(_ context.Context, tag string) error {
	a.calls = append(a.calls, "remove:"+tag)
	if tag == a.failRemoveTag {
		return errors.New("remove failed")
	}
	return nil
}
