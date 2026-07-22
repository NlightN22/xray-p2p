package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func TestExternalSubscriptionLastApplyTracksRuntimeOnly(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(externalTrojanFixture("fixture-password", "edge.example")))
	}))
	defer server.Close()
	opts := ExternalSubscriptionOptions{ID: "fixture", URL: server.URL, AllowHTTP: true}
	if err := AddExternalSubscription(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	statuses, err := ListExternalSubscriptions()
	if err != nil {
		t.Fatal(err)
	}
	if !statuses[0].LastApplyAt.IsZero() {
		t.Fatalf("staged refresh recorded runtime apply: %s", statuses[0].LastApplyAt)
	}

	stubRuntimeFlow(t, true, func(context.Context, xraylive.Options, xraylive.Artifacts) (xraylive.RuntimeApplyResult, error) {
		return xraylive.RuntimeApplyApplied, nil
	})
	if err := RefreshExternalSubscription(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	statuses, err = ListExternalSubscriptions()
	if err != nil {
		t.Fatal(err)
	}
	if statuses[0].LastApplyAt.IsZero() {
		t.Fatal("successful runtime refresh did not record apply time")
	}
}
