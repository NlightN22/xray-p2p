package apply

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestReadRequestRepairsLiteralNewlineEscapes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "apply.request")

	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339Nano)
	payload := `{"id":"req-1","timestamp":"` + ts + `","role":"client"}` + "\\n"
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	req, exists, err := ReadRequest(path)
	if err != nil {
		t.Fatalf("ReadRequest: %v", err)
	}
	if !exists {
		t.Fatalf("expected request to exist")
	}
	if req.ID != "req-1" {
		t.Fatalf("id mismatch: %q", req.ID)
	}
	if req.Role != "client" {
		t.Fatalf("role mismatch: %q", req.Role)
	}
}

func TestWriteRequestReplacesSameRoleGeneration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apply.request")
	first := Request{ID: "first", Timestamp: time.Now().UTC(), Role: RoleClient}
	second := Request{ID: "second", Timestamp: time.Now().UTC(), Role: RoleClient}
	if err := WriteRequest(path, first, ""); err != nil {
		t.Fatalf("write first request: %v", err)
	}
	if err := WriteRequest(path, second, ""); err != nil {
		t.Fatalf("write second request: %v", err)
	}
	got, exists, err := ReadRequest(path)
	if err != nil || !exists {
		t.Fatalf("read request: exists=%v err=%v", exists, err)
	}
	if got.ID != second.ID {
		t.Fatalf("request ID = %q, want %q", got.ID, second.ID)
	}
}

func TestWriteRequestPreservesIndependentRoleGenerations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apply.request")
	client := Request{ID: "client-generation", Role: RoleClient}
	server := Request{ID: "server-generation", Role: RoleServer}
	if err := WriteRequest(path, client, ""); err != nil {
		t.Fatalf("write client request: %v", err)
	}
	if err := WriteRequest(path, server, ""); err != nil {
		t.Fatalf("write server request: %v", err)
	}
	assertRoleRequest(t, path, RoleClient, client.ID)
	assertRoleRequest(t, path, RoleServer, server.ID)
}

func TestWriteAnyRequestCreatesIndependentGenerations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apply.request")
	if err := WriteRequest(path, Request{ID: "import-generation", Role: RoleAny}, ""); err != nil {
		t.Fatalf("write any request: %v", err)
	}
	client, _, err := ReadRequestForRole(path, RoleClient)
	if err != nil {
		t.Fatal(err)
	}
	server, _, err := ReadRequestForRole(path, RoleServer)
	if err != nil {
		t.Fatal(err)
	}
	if client.ID == server.ID {
		t.Fatalf("role generations share ID %q", client.ID)
	}
}

func TestReadRequestForRoleMigratesLegacyAny(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apply.request")
	legacy := `{"id":"legacy","timestamp":"2026-01-02T03:04:05Z","role":"any"}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	client, exists, err := ReadRequestForRole(path, RoleClient)
	if err != nil || !exists {
		t.Fatalf("read migrated client request: exists=%v err=%v", exists, err)
	}
	server, exists, err := ReadRequestForRole(path, RoleServer)
	if err != nil || !exists {
		t.Fatalf("read migrated server request: exists=%v err=%v", exists, err)
	}
	if client.ID != "legacy" || server.ID == client.ID {
		t.Fatalf("migrated IDs: client=%q server=%q", client.ID, server.ID)
	}
}

func TestReadRequestRejectsUnknownDocumentVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apply.request")
	if err := os.WriteFile(path, []byte(`{"version":3,"requests":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadRequestForRole(path, RoleClient); err == nil {
		t.Fatal("unknown request document version was accepted")
	}
}

func TestWriteRequestSerializesConcurrentRoles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apply.request")
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, req := range []Request{{ID: "client", Role: RoleClient}, {ID: "server", Role: RoleServer}} {
		req := req
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- WriteRequest(path, req, "")
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent write: %v", err)
		}
	}
	assertRoleRequest(t, path, RoleClient, "client")
	assertRoleRequest(t, path, RoleServer, "server")
}

func assertRoleRequest(t *testing.T, path, role, wantID string) {
	t.Helper()
	req, exists, err := ReadRequestForRole(path, role)
	if err != nil || !exists {
		t.Fatalf("read %s request: exists=%v err=%v", role, exists, err)
	}
	if req.ID != wantID {
		t.Fatalf("%s request ID = %q, want %q", role, req.ID, wantID)
	}
}
