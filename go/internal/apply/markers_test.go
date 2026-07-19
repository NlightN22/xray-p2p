package apply

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveRoleMarkersRemovesRoleSpecificFiles(t *testing.T) {
	tmp := t.TempDir()
	requestPath := filepath.Join(tmp, "apply.request")
	errorPath := filepath.Join(tmp, "apply.error")

	req, err := NewRequest(RoleClient)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if err := WriteRequest(requestPath, req, ""); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}
	if err := WriteError(errorPath, ErrorMarker{RequestID: req.ID, Role: RoleClient, Reason: "boom"}, ""); err != nil {
		t.Fatalf("WriteError: %v", err)
	}

	if err := RemoveRoleMarkers(requestPath, errorPath, RoleClient); err != nil {
		t.Fatalf("RemoveRoleMarkers: %v", err)
	}

	if _, err := os.Stat(requestPath); !os.IsNotExist(err) {
		t.Fatalf("expected request removed, stat err=%v", err)
	}
	if _, err := os.Stat(errorPath); !os.IsNotExist(err) {
		t.Fatalf("expected error removed, stat err=%v", err)
	}
}

func TestRemoveRoleMarkersPreservesAnyRequest(t *testing.T) {
	tmp := t.TempDir()
	requestPath := filepath.Join(tmp, "apply.request")
	errorPath := filepath.Join(tmp, "apply.error")

	req, err := NewRequest(RoleAny)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if err := WriteRequest(requestPath, req, ""); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	if err := RemoveRoleMarkers(requestPath, errorPath, RoleClient); err != nil {
		t.Fatalf("RemoveRoleMarkers: %v", err)
	}
	if _, err := os.Stat(requestPath); err != nil {
		t.Fatalf("expected request preserved, stat err=%v", err)
	}
}

func TestCompleteRequestPreservesNewerRequest(t *testing.T) {
	tmp := t.TempDir()
	requestPath := filepath.Join(tmp, "apply.request")
	errorPath := filepath.Join(tmp, "apply.error")
	old := Request{ID: "old", Role: RoleClient}
	newer := Request{ID: "new", Role: RoleClient}
	if err := WriteRequest(requestPath, newer, ""); err != nil {
		t.Fatalf("write newer request: %v", err)
	}
	if err := CompleteRequest(requestPath, errorPath, old); err != nil {
		t.Fatalf("complete old request: %v", err)
	}
	got, exists, err := ReadRequest(requestPath)
	if err != nil || !exists {
		t.Fatalf("read newer request: exists=%v err=%v", exists, err)
	}
	if got.ID != newer.ID {
		t.Fatalf("request ID = %q, want %q", got.ID, newer.ID)
	}
}

func TestCompleteRequestRemovesExactRequestAndError(t *testing.T) {
	tmp := t.TempDir()
	requestPath := filepath.Join(tmp, "apply.request")
	errorPath := filepath.Join(tmp, "apply.error")
	req := Request{ID: "current", Role: RoleClient}
	if err := WriteRequest(requestPath, req, ""); err != nil {
		t.Fatalf("write request: %v", err)
	}
	if err := WriteError(errorPath, ErrorMarker{RequestID: req.ID, Role: RoleClient, Reason: "old"}, ""); err != nil {
		t.Fatalf("write error: %v", err)
	}
	if err := CompleteRequest(requestPath, errorPath, req); err != nil {
		t.Fatalf("complete request: %v", err)
	}
	if _, exists, err := ReadRequest(requestPath); err != nil || exists {
		t.Fatalf("request remains: exists=%v err=%v", exists, err)
	}
	if _, exists, err := ReadError(errorPath); err != nil || exists {
		t.Fatalf("error remains: exists=%v err=%v", exists, err)
	}
}
