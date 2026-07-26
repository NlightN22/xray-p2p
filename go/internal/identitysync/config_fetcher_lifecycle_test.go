//go:build windows || linux

package identitysync

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestSCIMSnapshotReusesOneClient(t *testing.T) {
	var connections atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users":
			_, _ = w.Write([]byte(`{"totalResults":1,"startIndex":1,"itemsPerPage":1,"Resources":[{"id":"u1","userName":"alice"}]}`))
		case "/groups":
			_, _ = w.Write([]byte(`{"totalResults":1,"startIndex":1,"itemsPerPage":1,"Resources":[{"id":"g1","name":"engineering"}]}`))
		case "/groups/g1/members":
			_, _ = w.Write([]byte(`{"totalResults":1,"startIndex":1,"itemsPerPage":1,"Resources":[{"id":"u1","userName":"alice"}]}`))
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewUnstartedServer(handler)
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	defer server.Close()

	fetcher := ConfigFetcher{Config: config.IdentityProviderConfig{
		SCIM: config.SCIMProviderConfig{Endpoint: server.URL},
	}}
	snapshot, err := fetcher.FetchSnapshot(t.Context(), ProviderRef{InstanceID: "corp", Kind: ProviderSCIM})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Complete || len(snapshot.Subjects) != 1 || len(snapshot.Groups) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if got := connections.Load(); got != 1 {
		t.Fatalf("SCIM snapshot opened %d connections, want 1", got)
	}
}
