package subscription

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSourceConditionalFetchAndLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"rev-1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"rev-1"`)
		_, _ = w.Write([]byte(trojanFixture))
	}))
	defer server.Close()
	source := HTTPSource{SourceRef: SourceRef{ID: "external-main"}, URL: server.URL, AllowHTTP: true, MaxBytes: 4096}
	raw, err := source.Fetch(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if raw.Revision != `"rev-1"` || string(raw.Data) != trojanFixture {
		t.Fatalf("unexpected raw snapshot: %+v", raw)
	}
	if _, err := source.Fetch(context.Background(), raw.Revision); !errors.Is(err, ErrNotModified) {
		t.Fatalf("conditional fetch error = %v", err)
	}
	source.MaxBytes = 8
	if _, err := source.Fetch(context.Background(), ""); err == nil {
		t.Fatal("oversized response accepted")
	}
}

func TestHTTPSourceRequiresHTTPS(t *testing.T) {
	_, err := (HTTPSource{URL: "http://example.test/subscription"}).Fetch(context.Background(), "")
	if err == nil {
		t.Fatal("HTTP subscription accepted by default")
	}
}
