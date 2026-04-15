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

