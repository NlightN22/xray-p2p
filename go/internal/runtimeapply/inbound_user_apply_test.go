package runtimeapply

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestApplyInboundUserDiffSequencesRemoveBeforeAdd(t *testing.T) {
	applier := newRecordingInboundUserApplier()
	applier.users["trojan-in"] = map[string]string{"old@example.com": "old"}
	diff := Diff{
		Kind: DiffInboundUsers,
		RemovedInboundUsers: []InboundUserChange{
			{InboundTag: "trojan-in", Email: "old@example.com", Password: "old"},
		},
		AddedInboundUsers: []InboundUserChange{
			{InboundTag: "trojan-in", Email: "new@example.com", Password: "new"},
		},
	}

	if err := ApplyInboundUserDiff(context.Background(), applier, diff); err != nil {
		t.Fatalf("ApplyInboundUserDiff: %v", err)
	}
	want := []string{"remove:trojan-in:old@example.com", "add:trojan-in:new@example.com"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

func TestApplyInboundUserDiffRollsBackAddedUsers(t *testing.T) {
	applier := newRecordingInboundUserApplier()
	applier.failAddEmail = "bad@example.com"
	diff := Diff{
		Kind: DiffInboundUsers,
		AddedInboundUsers: []InboundUserChange{
			{InboundTag: "trojan-in", Email: "good@example.com", Password: "good"},
			{InboundTag: "trojan-in", Email: "bad@example.com", Password: "bad"},
		},
	}

	err := ApplyInboundUserDiff(context.Background(), applier, diff)
	if err == nil {
		t.Fatal("expected error")
	}
	want := []string{"add:trojan-in:good@example.com", "add:trojan-in:bad@example.com", "remove:trojan-in:good@example.com"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

func TestApplyInboundUserDiffRestoresRemovedUsers(t *testing.T) {
	applier := newRecordingInboundUserApplier()
	applier.users["trojan-in"] = map[string]string{"good@example.com": "good", "bad@example.com": "bad"}
	applier.failRemoveEmail = "bad@example.com"
	diff := Diff{
		Kind: DiffInboundUsers,
		RemovedInboundUsers: []InboundUserChange{
			{InboundTag: "trojan-in", Email: "good@example.com", Password: "good"},
			{InboundTag: "trojan-in", Email: "bad@example.com", Password: "bad"},
		},
	}

	err := ApplyInboundUserDiff(context.Background(), applier, diff)
	if err == nil {
		t.Fatal("expected error")
	}
	want := []string{"remove:trojan-in:good@example.com", "remove:trojan-in:bad@example.com", "add:trojan-in:good@example.com"}
	if !reflect.DeepEqual(applier.calls, want) {
		t.Fatalf("calls = %v, want %v", applier.calls, want)
	}
}

type recordingInboundUserApplier struct {
	calls           []string
	users           map[string]map[string]string
	failAddEmail    string
	failRemoveEmail string
}

func newRecordingInboundUserApplier() *recordingInboundUserApplier {
	return &recordingInboundUserApplier{users: make(map[string]map[string]string)}
}

func (a *recordingInboundUserApplier) AddInboundUser(_ context.Context, inboundTag, email, password string) error {
	a.calls = append(a.calls, "add:"+inboundTag+":"+email)
	if email == a.failAddEmail {
		return errors.New("add failed")
	}
	if a.users[inboundTag] == nil {
		a.users[inboundTag] = make(map[string]string)
	}
	a.users[inboundTag][email] = password
	return nil
}

func (a *recordingInboundUserApplier) RemoveInboundUser(_ context.Context, inboundTag, email string) error {
	a.calls = append(a.calls, "remove:"+inboundTag+":"+email)
	if email == a.failRemoveEmail {
		return errors.New("remove failed")
	}
	delete(a.users[inboundTag], email)
	return nil
}

func (a *recordingInboundUserApplier) ListInboundUserEmails(_ context.Context, inboundTag string) ([]string, error) {
	users := a.users[inboundTag]
	result := make([]string, 0, len(users))
	for email := range users {
		result = append(result, email)
	}
	return result, nil
}
