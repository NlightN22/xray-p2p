package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/subscription"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

func TestExternalSubscriptionRefreshPersistsLKGAndDesiredOffers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	body := trojanExternalFixture
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"fixture-1"`)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	if err := AddExternalSubscription(context.Background(), ExternalSubscriptionOptions{ID: "fixture", URL: server.URL, AllowHTTP: true}); err != nil {
		t.Fatal(err)
	}
	desired, err := loadClientInstallState(filepath.Join(root, "xp2p-client.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(desired.Subscriptions) != 1 || len(desired.Endpoints) != 1 || desired.Endpoints[0].Password != "fixture-password" {
		t.Fatalf("unexpected Desired state: %+v", desired)
	}
	if _, err := os.Stat(filepath.Join(root, ".state", "subscriptions", "fixture.json")); err != nil {
		t.Fatalf("LKG state missing: %v", err)
	}
	body = "malformed-snapshot"
	if err := RefreshExternalSubscription(context.Background(), ExternalSubscriptionOptions{ID: "fixture", AllowHTTP: true}); err == nil {
		t.Fatal("malformed refresh succeeded")
	}
	afterFailure, err := loadClientInstallState(filepath.Join(root, "xp2p-client.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(afterFailure.Endpoints) != 1 || afterFailure.Endpoints[0].Password != "fixture-password" {
		t.Fatalf("failed refresh changed Desired: %+v", afterFailure)
	}
}

const trojanExternalFixture = "trojan://fixture-password@edge.example:443?security=tls&type=tcp&sni=edge.example#Fixture"

func TestReplaceSubscriptionEndpointsIsSourceIsolated(t *testing.T) {
	current := []clientEndpointRecord{
		{Tag: "manual", Hostname: "manual.example"},
		{Tag: "source-a-old", SubscriptionSourceID: "source-a", SubscriptionOfferID: "old"},
		{Tag: "source-b", SubscriptionSourceID: "source-b", SubscriptionOfferID: "other"},
	}
	offers := []subscription.ConnectionOffer{{
		StableID:  "offer-0123456789abcdef01234567",
		Endpoint:  tunnel.Endpoint{Host: "edge.example", Port: 443, Profile: tunnel.ProfileTrojanTLS, Protocol: "trojan", Transport: "tcp", Security: "tls"},
		UserLabel: "fixture", Credential: "server-secret",
	}}
	got := replaceSubscriptionEndpoints(current, "source-a", offers[0].StableID, offers)
	if len(got) != 3 {
		t.Fatalf("endpoints = %d, want 3", len(got))
	}
	if got[0].Tag != "manual" || got[1].SubscriptionSourceID != "source-b" {
		t.Fatalf("unrelated endpoints changed: %+v", got)
	}
	added := got[2]
	if added.SubscriptionSourceID != "source-a" || added.SubscriptionOfferID != offers[0].StableID || added.Password != "server-secret" || added.Tag != "subscription-0123456789abcdef" {
		t.Fatalf("unexpected external endpoint: %+v", added)
	}
	if added.Disabled {
		t.Fatal("selected external endpoint is disabled")
	}
}

func TestLegacyDesiredEndpointsRemainUnowned(t *testing.T) {
	state := clientInstallState{Endpoints: []clientEndpointRecord{{Hostname: "legacy.example", Password: "legacy-secret"}}}
	state.normalize()
	if state.Endpoints[0].SubscriptionSourceID != "" || state.Endpoints[0].SubscriptionOfferID != "" {
		t.Fatalf("legacy endpoint acquired subscription ownership: %+v", state.Endpoints[0])
	}
	if state.Endpoints[0].Profile != "trojan-tls" || state.Endpoints[0].Protocol != "trojan" {
		t.Fatalf("legacy endpoint defaults changed: %+v", state.Endpoints[0])
	}
}

func TestSelectExternalOfferPreservesAvailableSelection(t *testing.T) {
	offers := []subscription.ConnectionOffer{{StableID: "offer-a"}, {StableID: "offer-b"}}
	if got := selectExternalOffer("offer-b", offers); got != "offer-b" {
		t.Fatalf("selection = %q, want preserved offer-b", got)
	}
	if got := selectExternalOffer("removed", offers); got != "offer-a" {
		t.Fatalf("fallback selection = %q, want offer-a", got)
	}
}
