package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
)

func TestControlStatusFaultRequiresTestMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status")
	if err := os.WriteFile(path, []byte("503"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(testControlStatusFileEnv, path)
	t.Setenv("XP2P_TEST_MODE", "1")
	handler := testControlFaultHandler(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, controlplane.PathSubscription, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
