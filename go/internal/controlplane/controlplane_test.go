package controlplane

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
)

func TestGenerationIgnoresVolatileTimes(t *testing.T) {
	base := Subscription{
		Profile:    "trojan-tls",
		Protocol:   "trojan",
		Transport:  "tcp",
		Security:   "tls",
		Host:       "edge.example",
		Port:       8443,
		ServerName: "edge.example",
	}
	first, err := BuildSubscription(base, time.Unix(100, 0), time.Minute)
	if err != nil {
		t.Fatalf("build subscription: %v", err)
	}
	second, err := BuildSubscription(base, time.Unix(200, 0), 2*time.Minute)
	if err != nil {
		t.Fatalf("build subscription: %v", err)
	}
	if first.Generation != second.Generation {
		t.Fatalf("generation changed for volatile fields: %s != %s", first.Generation, second.Generation)
	}
	second.Port = 9443
	changed, err := Generation(second)
	if err != nil {
		t.Fatalf("generation changed payload: %v", err)
	}
	if changed == first.Generation {
		t.Fatalf("generation did not change for canonical payload change")
	}
}

func TestVerifyRequestHMAC(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	body := []byte(`{"nonce":"n1"}`)
	req := httptest.NewRequest(http.MethodPost, PathPing, bytes.NewReader(body))
	if err := ApplyHeaders(req, "alice", "secret", "n1", body, now); err != nil {
		t.Fatalf("apply headers: %v", err)
	}
	if err := VerifyRequest(req, body, []AuthUser{{Label: "alice", Credential: "secret"}}, now, 120*time.Second); err != nil {
		t.Fatalf("verify request: %v", err)
	}
	req.Header.Set(HeaderSignature, "00")
	if err := VerifyRequest(req, body, []AuthUser{{Label: "alice", Credential: "secret"}}, now, 120*time.Second); err == nil {
		t.Fatalf("expected invalid signature")
	}
}

func TestHandlersServePingHeartbeatAndSubscription(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	store, err := heartbeat.NewStore("")
	if err != nil {
		t.Fatalf("heartbeat store: %v", err)
	}
	runtime := Runtime{
		Subscription: Subscription{
			Generation: "gen1",
			Profile:    "trojan-tls",
			Protocol:   "trojan",
			Transport:  "tcp",
			Security:   "tls",
			Host:       "edge.example",
			Port:       8443,
		},
		AuthUsers: []AuthUser{{Label: "alice", Credential: "secret"}},
	}
	handler := NewHandler(HandlerOptions{
		LoadRuntime: func() (Runtime, error) { return runtime, nil },
		Heartbeat:   store,
		Now:         func() time.Time { return now },
	})

	pingBody := []byte(`{"nonce":"n1"}`)
	pingReq := signedRequest(t, http.MethodPost, PathPing, pingBody, now)
	pingResp := httptest.NewRecorder()
	handler.ServeHTTP(pingResp, pingReq)
	if pingResp.Code != http.StatusOK {
		t.Fatalf("ping status = %d body=%s", pingResp.Code, pingResp.Body.String())
	}
	var pong PingResponse
	if err := json.NewDecoder(pingResp.Body).Decode(&pong); err != nil {
		t.Fatalf("decode pong: %v", err)
	}
	if pong.Nonce != "n1" {
		t.Fatalf("nonce = %q", pong.Nonce)
	}

	hbBody := []byte(`{"tag":"proxy-a","host":"edge.example","rtt_ms":7}`)
	hbReq := signedRequest(t, http.MethodPost, PathHeartbeat, hbBody, now)
	hbResp := httptest.NewRecorder()
	handler.ServeHTTP(hbResp, hbReq)
	if hbResp.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d body=%s", hbResp.Code, hbResp.Body.String())
	}
	if snapshots := store.Snapshot(now, time.Minute); len(snapshots) != 1 {
		t.Fatalf("heartbeat snapshots = %d", len(snapshots))
	}

	subReq := signedRequest(t, http.MethodGet, PathSubscription, nil, now)
	subResp := httptest.NewRecorder()
	handler.ServeHTTP(subResp, subReq)
	if subResp.Code != http.StatusOK {
		t.Fatalf("subscription status = %d body=%s", subResp.Code, subResp.Body.String())
	}
	var sub Subscription
	if err := json.NewDecoder(subResp.Body).Decode(&sub); err != nil {
		t.Fatalf("decode subscription: %v", err)
	}
	if sub.Generation == "" {
		t.Fatalf("generation missing")
	}
}

func signedRequest(t *testing.T, method, path string, body []byte, now time.Time) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if err := ApplyHeaders(req, "alice", "secret", "nonce-"+method+path, body, now); err != nil {
		t.Fatalf("apply headers: %v", err)
	}
	return req
}
