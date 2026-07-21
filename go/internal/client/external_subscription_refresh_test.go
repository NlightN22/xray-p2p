package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/subscription"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func TestExternalSubscriptionRefreshReconcilesServerSnapshot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	body := externalTwoOfferFixture("old-trojan-password", "550e8400-e29b-41d4-a716-446655440000")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	opts := ExternalSubscriptionOptions{ID: "fixture", URL: server.URL, AllowHTTP: true}
	if err := AddExternalSubscription(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	before := mustLoadExternalDesired(t, root)
	if len(before.Endpoints) != 2 {
		t.Fatalf("initial endpoints = %d, want 2", len(before.Endpoints))
	}
	trojanOfferID := endpointOfferID(before, "trojan")

	body = externalTrojanFixture("new-trojan-password", "new-edge.example")
	if err := RefreshExternalSubscription(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	after := mustLoadExternalDesired(t, root)
	if len(after.Endpoints) != 1 {
		t.Fatalf("refreshed endpoints = %d, want 1", len(after.Endpoints))
	}
	endpoint := after.Endpoints[0]
	if endpoint.Protocol != "trojan" || endpoint.Password != "new-trojan-password" || endpoint.ServerName != "new-edge.example" {
		t.Fatalf("server snapshot was not applied verbatim: %+v", endpoint)
	}
	if endpoint.SubscriptionOfferID == trojanOfferID {
		t.Fatal("security metadata change did not change stable offer identity")
	}
}

func TestExternalSubscriptionFailurePreservesLKGAndRestartRefreshes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	body := externalTrojanFixture("first-password", "edge.example")
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	opts := ExternalSubscriptionOptions{ID: "fixture", URL: server.URL, AllowHTTP: true}
	if err := AddExternalSubscription(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	fail = true
	if err := RefreshExternalSubscription(context.Background(), ExternalSubscriptionOptions{ID: "fixture", AllowHTTP: true}); err == nil {
		t.Fatal("unavailable subscription refresh succeeded")
	}
	failed := mustLoadExternalDesired(t, root)
	if len(failed.Endpoints) != 1 || failed.Endpoints[0].Password != "first-password" {
		t.Fatalf("failed refresh changed Desired: %+v", failed.Endpoints)
	}
	statuses, err := ListExternalSubscriptions()
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || len(statuses[0].Offers) != 1 || statuses[0].LastError == "" {
		t.Fatalf("LKG diagnostic missing: %+v", statuses)
	}

	fail = false
	body = externalTrojanFixture("after-restart-password", "edge.example")
	if err := RefreshExternalSubscription(context.Background(), ExternalSubscriptionOptions{ID: "fixture", AllowHTTP: true}); err != nil {
		t.Fatal(err)
	}
	restarted := mustLoadExternalDesired(t, root)
	if restarted.Endpoints[0].Password != "after-restart-password" {
		t.Fatalf("persisted source did not refresh after restart: %+v", restarted.Endpoints[0])
	}
}

func TestRemoveExternalSubscriptionPreservesManualEndpoints(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(externalTrojanFixture("fixture-password", "edge.example")))
	}))
	defer server.Close()

	state := clientInstallState{Endpoints: []clientEndpointRecord{{Tag: "manual", Hostname: "manual.example"}}}
	if err := state.save(filepath.Join(root, "xp2p-client.toml")); err != nil {
		t.Fatal(err)
	}
	opts := ExternalSubscriptionOptions{ID: "fixture", URL: server.URL, AllowHTTP: true}
	if err := AddExternalSubscription(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if err := RemoveExternalSubscription(context.Background(), "fixture"); err != nil {
		t.Fatal(err)
	}
	after := mustLoadExternalDesired(t, root)
	if len(after.Subscriptions) != 0 || len(after.Endpoints) != 1 || after.Endpoints[0].Tag != "manual" {
		t.Fatalf("remove changed unrelated Desired state: %+v", after)
	}
	if statuses, err := ListExternalSubscriptions(); err != nil || len(statuses) != 0 {
		t.Fatalf("removed subscription remains visible: %+v, %v", statuses, err)
	}
}

func TestExternalSubscriptionRuntimeFailureKeepsPreviousDesiredAndLKG(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	body := externalTrojanFixture("old-password", "edge.example")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	opts := ExternalSubscriptionOptions{ID: "fixture", URL: server.URL, AllowHTTP: true}
	if err := AddExternalSubscription(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	body = externalTrojanFixture("new-password", "edge.example")
	stubRuntimeFlow(t, true, func(context.Context, xraylive.Options, xraylive.Artifacts) (xraylive.RuntimeApplyResult, error) {
		return xraylive.RuntimeApplyFailed, nil
	})
	if err := RefreshExternalSubscription(context.Background(), opts); err == nil {
		t.Fatal("failed runtime apply reported success")
	}
	assertExternalCredential(t, root, "old-password")
}

func TestExternalSubscriptionLKGFailureRestoresDesired(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	body := externalTrojanFixture("old-password", "edge.example")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	opts := ExternalSubscriptionOptions{ID: "fixture", URL: server.URL, AllowHTTP: true}
	if err := AddExternalSubscription(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	body = externalTrojanFixture("new-password", "edge.example")
	oldSave := saveExternalSubscriptionState
	saveExternalSubscriptionState = func(store subscription.Store, state subscription.PersistedSource, digest string) error {
		if state.LastGood != nil && state.LastGood.Offers[0].Credential == "new-password" {
			return errors.New("injected LKG failure")
		}
		return store.Save(state, digest)
	}
	t.Cleanup(func() { saveExternalSubscriptionState = oldSave })
	if err := RefreshExternalSubscription(context.Background(), opts); err == nil {
		t.Fatal("failed LKG persistence reported success")
	}
	assertExternalCredential(t, root, "old-password")
}

func TestExternalSubscriptionRefreshRejectsConcurrentDesiredChange(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	desiredPath := filepath.Join(root, "xp2p-client.toml")
	body := externalTrojanFixture("old-password", "edge.example")
	mutateDesired := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if mutateDesired {
			current, err := os.ReadFile(desiredPath)
			if err != nil {
				t.Errorf("read concurrent Desired: %v", err)
			} else if err := os.WriteFile(desiredPath, append(current, []byte("\n# concurrent edit\n")...), 0o600); err != nil {
				t.Errorf("write concurrent Desired: %v", err)
			}
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	opts := ExternalSubscriptionOptions{ID: "fixture", URL: server.URL, AllowHTTP: true}
	if err := AddExternalSubscription(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	body = externalTrojanFixture("new-password", "edge.example")
	mutateDesired = true
	err := RefreshExternalSubscription(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "changed concurrently") {
		t.Fatalf("refresh error = %v, want concurrent Desired conflict", err)
	}
	assertExternalCredential(t, root, "old-password")
}

func TestAddExternalSubscriptionFetchFailureLeavesNoSource(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	err := AddExternalSubscription(context.Background(), ExternalSubscriptionOptions{ID: "fixture", URL: server.URL, AllowHTTP: true})
	if err == nil {
		t.Fatal("failed initial fetch reported success")
	}
	state := mustLoadExternalDesired(t, root)
	if len(state.Subscriptions) != 0 || len(state.Endpoints) != 0 {
		t.Fatalf("failed add changed Desired: %+v", state)
	}
	if statuses, listErr := ListExternalSubscriptions(); listErr != nil || len(statuses) != 0 {
		t.Fatalf("failed add left persisted source: %+v, %v", statuses, listErr)
	}
}

func assertExternalCredential(t *testing.T, root, credential string) {
	t.Helper()
	desired := mustLoadExternalDesired(t, root)
	if len(desired.Endpoints) != 1 || desired.Endpoints[0].Password != credential {
		t.Fatalf("Desired credential changed: %+v", desired.Endpoints)
	}
	statuses, err := ListExternalSubscriptions()
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || len(statuses[0].Offers) != 1 || statuses[0].Offers[0].Credential != credential {
		t.Fatalf("LKG credential changed: %+v", statuses)
	}
}

func mustLoadExternalDesired(t *testing.T, root string) clientInstallState {
	t.Helper()
	state, err := loadClientInstallState(filepath.Join(root, "xp2p-client.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func endpointOfferID(state clientInstallState, protocol string) string {
	for _, endpoint := range state.Endpoints {
		if endpoint.Protocol == protocol {
			return endpoint.SubscriptionOfferID
		}
	}
	return ""
}

func externalTwoOfferFixture(trojanPassword, vlessID string) string {
	return strings.Join([]string{
		externalTrojanFixture(trojanPassword, "edge.example"),
		"vless://" + vlessID + "@edge.example:443?security=tls&type=tcp&flow=xtls-rprx-vision&sni=edge.example#VLESS",
	}, "\n")
}

func externalTrojanFixture(password, serverName string) string {
	return "trojan://" + password + "@edge.example:443?security=tls&type=tcp&sni=" + serverName + "#Trojan"
}
