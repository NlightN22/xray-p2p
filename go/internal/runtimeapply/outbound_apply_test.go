package runtimeapply

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestApplyOutboundDiffSequencesRemoveBeforeAdd(t *testing.T) {
	applier := newRecordingOutboundApplier("old")
	diff := Diff{
		Kind: DiffOutboundOnly,
		RemovedOutbounds: []OutboundChange{
			{Tag: "old", Outbound: map[string]any{"tag": "old"}},
		},
		AddedOutbounds: []OutboundChange{
			{Tag: "new", Outbound: map[string]any{"tag": "new"}},
		},
	}

	if err := ApplyOutboundDiff(context.Background(), applier, diff); err != nil {
		t.Fatalf("ApplyOutboundDiff: %v", err)
	}
	want := []string{"remove:old", "add:new"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

func TestApplyOutboundDiffRollsBackAddedOutbounds(t *testing.T) {
	applier := newRecordingOutboundApplier()
	applier.failAddTag = "bad"
	diff := Diff{
		Kind: DiffOutboundOnly,
		AddedOutbounds: []OutboundChange{
			{Tag: "good", Outbound: map[string]any{"tag": "good"}},
			{Tag: "bad", Outbound: map[string]any{"tag": "bad"}},
		},
	}

	err := ApplyOutboundDiff(context.Background(), applier, diff)
	if err == nil {
		t.Fatal("expected error")
	}
	want := []string{"add:good", "add:bad", "remove:good"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

func TestApplyOutboundDiffRestoresRemovedOutbounds(t *testing.T) {
	applier := newRecordingOutboundApplier("good", "bad")
	applier.failRemoveTag = "bad"
	diff := Diff{
		Kind: DiffOutboundOnly,
		RemovedOutbounds: []OutboundChange{
			{Tag: "good", Outbound: map[string]any{"tag": "good"}},
			{Tag: "bad", Outbound: map[string]any{"tag": "bad"}},
		},
	}

	err := ApplyOutboundDiff(context.Background(), applier, diff)
	if err == nil {
		t.Fatal("expected error")
	}
	want := []string{"remove:good", "remove:bad", "add:good"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

type recordingOutboundApplier struct {
	calls         []string
	tags          map[string]struct{}
	failAddTag    string
	failRemoveTag string
}

func newRecordingOutboundApplier(tags ...string) *recordingOutboundApplier {
	applier := &recordingOutboundApplier{tags: make(map[string]struct{}, len(tags))}
	for _, tag := range tags {
		applier.tags[tag] = struct{}{}
	}
	return applier
}

func (a *recordingOutboundApplier) AddOutbound(_ context.Context, outbound map[string]any) error {
	tag, _ := outbound["tag"].(string)
	a.calls = append(a.calls, "add:"+tag)
	if tag == a.failAddTag {
		return errors.New("add failed")
	}
	a.tags[tag] = struct{}{}
	return nil
}

func (a *recordingOutboundApplier) RemoveOutbound(_ context.Context, tag string) error {
	a.calls = append(a.calls, "remove:"+tag)
	if tag == a.failRemoveTag {
		return errors.New("remove failed")
	}
	delete(a.tags, tag)
	return nil
}

func (a *recordingOutboundApplier) ListOutboundTags(context.Context) ([]string, error) {
	result := make([]string, 0, len(a.tags))
	for tag := range a.tags {
		result = append(result, tag)
	}
	return result, nil
}
