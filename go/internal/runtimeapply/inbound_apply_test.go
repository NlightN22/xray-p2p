package runtimeapply

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestApplyInboundDiffSequencesRemoveBeforeAdd(t *testing.T) {
	applier := newRecordingInboundApplier("old")
	diff := Diff{
		Kind: DiffInboundOnly,
		RemovedInbounds: []InboundChange{
			{Tag: "old", Inbound: map[string]any{"tag": "old"}},
		},
		AddedInbounds: []InboundChange{
			{Tag: "new", Inbound: map[string]any{"tag": "new"}},
		},
	}

	if err := ApplyInboundDiff(context.Background(), applier, diff); err != nil {
		t.Fatalf("ApplyInboundDiff: %v", err)
	}
	want := []string{"remove:old", "add:new"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

func TestApplyInboundDiffRollsBackAddedInbounds(t *testing.T) {
	applier := newRecordingInboundApplier()
	applier.failAddTag = "bad"
	diff := Diff{
		Kind: DiffInboundOnly,
		AddedInbounds: []InboundChange{
			{Tag: "good", Inbound: map[string]any{"tag": "good"}},
			{Tag: "bad", Inbound: map[string]any{"tag": "bad"}},
		},
	}

	err := ApplyInboundDiff(context.Background(), applier, diff)
	if err == nil {
		t.Fatal("expected error")
	}
	want := []string{"add:good", "add:bad", "remove:good"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

func TestApplyInboundDiffRestoresRemovedInbounds(t *testing.T) {
	applier := newRecordingInboundApplier("good", "bad")
	applier.failRemoveTag = "bad"
	diff := Diff{
		Kind: DiffInboundOnly,
		RemovedInbounds: []InboundChange{
			{Tag: "good", Inbound: map[string]any{"tag": "good"}},
			{Tag: "bad", Inbound: map[string]any{"tag": "bad"}},
		},
	}

	err := ApplyInboundDiff(context.Background(), applier, diff)
	if err == nil {
		t.Fatal("expected error")
	}
	want := []string{"remove:good", "remove:bad", "add:good"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

type recordingInboundApplier struct {
	calls         []string
	tags          map[string]struct{}
	failAddTag    string
	failRemoveTag string
}

func newRecordingInboundApplier(tags ...string) *recordingInboundApplier {
	applier := &recordingInboundApplier{tags: make(map[string]struct{}, len(tags))}
	for _, tag := range tags {
		applier.tags[tag] = struct{}{}
	}
	return applier
}

func (a *recordingInboundApplier) AddInbound(_ context.Context, inbound map[string]any) error {
	tag, _ := inbound["tag"].(string)
	a.calls = append(a.calls, "add:"+tag)
	if tag == a.failAddTag {
		return errors.New("add failed")
	}
	a.tags[tag] = struct{}{}
	return nil
}

func (a *recordingInboundApplier) RemoveInbound(_ context.Context, tag string) error {
	a.calls = append(a.calls, "remove:"+tag)
	if tag == a.failRemoveTag {
		return errors.New("remove failed")
	}
	delete(a.tags, tag)
	return nil
}

func (a *recordingInboundApplier) ListInboundTags(context.Context) ([]string, error) {
	result := make([]string, 0, len(a.tags))
	for tag := range a.tags {
		result = append(result, tag)
	}
	return result, nil
}
