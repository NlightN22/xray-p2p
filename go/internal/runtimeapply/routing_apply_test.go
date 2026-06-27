package runtimeapply

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestApplyRoutingDiffSequencesRemoveBeforeAdd(t *testing.T) {
	applier := newRecordingRoutingApplier("old")
	diff := Diff{
		Kind:              DiffRoutingOnly,
		CandidateRuleTags: []string{"new"},
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
	applier := newRecordingRoutingApplier()
	applier.failAddTag = "bad"
	diff := Diff{
		Kind:              DiffRoutingOnly,
		CandidateRuleTags: []string{"good", "bad"},
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
	applier := newRecordingRoutingApplier("good", "bad")
	applier.failRemoveTag = "bad"
	diff := Diff{
		Kind:              DiffRoutingOnly,
		CandidateRuleTags: []string{},
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

func TestApplyRoutingDiffAllowsSuffixRewriteReadd(t *testing.T) {
	applier := newRecordingRoutingApplier("keep", "wide")
	diff := Diff{
		Kind:              DiffRoutingOnly,
		CandidateRuleTags: []string{"keep", "narrow", "wide"},
		RemovedRules: []RoutingRuleChange{
			{RuleTag: "wide", Rule: map[string]any{"ruleTag": "wide"}},
		},
		AddedRules: []RoutingRuleChange{
			{RuleTag: "narrow", Rule: map[string]any{"ruleTag": "narrow"}},
			{RuleTag: "wide", Rule: map[string]any{"ruleTag": "wide"}},
		},
	}

	if err := ApplyRoutingDiff(context.Background(), applier, diff); err != nil {
		t.Fatalf("ApplyRoutingDiff: %v", err)
	}
	want := []string{"remove:wide", "add:narrow", "add:wide"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
	if !reflect.DeepEqual(applier.order, diff.CandidateRuleTags) {
		t.Fatalf("order = %v, want %v", applier.order, diff.CandidateRuleTags)
	}
}

type recordingRoutingApplier struct {
	calls         []string
	tags          map[string]struct{}
	order         []string
	failAddTag    string
	failRemoveTag string
}

func newRecordingRoutingApplier(tags ...string) *recordingRoutingApplier {
	applier := &recordingRoutingApplier{
		tags:  make(map[string]struct{}, len(tags)),
		order: append([]string(nil), tags...),
	}
	for _, tag := range tags {
		applier.tags[tag] = struct{}{}
	}
	return applier
}

func (a *recordingRoutingApplier) AddRule(_ context.Context, rule map[string]any) error {
	tag, _ := rule["ruleTag"].(string)
	a.calls = append(a.calls, "add:"+tag)
	if tag == a.failAddTag {
		return errors.New("add failed")
	}
	a.tags[tag] = struct{}{}
	a.order = append(a.order, tag)
	return nil
}

func (a *recordingRoutingApplier) RemoveRule(_ context.Context, tag string) error {
	a.calls = append(a.calls, "remove:"+tag)
	if tag == a.failRemoveTag {
		return errors.New("remove failed")
	}
	delete(a.tags, tag)
	a.order = removeString(a.order, tag)
	return nil
}

func (a *recordingRoutingApplier) ListRuleTags(context.Context) ([]string, error) {
	result := make([]string, 0, len(a.order))
	for _, tag := range a.order {
		if _, ok := a.tags[tag]; ok {
			result = append(result, tag)
		}
	}
	return result, nil
}

func removeString(items []string, value string) []string {
	result := items[:0]
	for _, item := range items {
		if item != value {
			result = append(result, item)
		}
	}
	return result
}
