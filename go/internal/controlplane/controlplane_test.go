package controlplane

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestBuildSubscriptionUsesClientVisibleTLSMetadata(t *testing.T) {
	base := Subscription{
		Profile:    "trojan-tls",
		Protocol:   "trojan",
		Transport:  "tcp",
		Security:   "tls",
		Host:       "edge.example",
		Port:       58443,
		ServerName: "edge.example",
		TLS: TLSMetadata{
			ServerName:             "edge.example",
			PinnedPeerCertSHA256:   "pin",
			VerifyPeerCertByName:   "edge.example",
			ClientMayAllowInsecure: false,
		},
		Parameters: map[string]string{"tunnel_port": "58443"},
	}
	serverOnly := base
	serverOnly.TLS.CertificatePath = "/etc/xp2p/cert.pem"
	serverOnly.TLS.SelfSigned = true

	first, err := BuildSubscription(base, time.Unix(100, 0), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildSubscription(serverOnly, time.Unix(100, 0), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if second.TLS.CertificatePath != "" || second.TLS.SelfSigned {
		t.Fatalf("server-only TLS metadata leaked into subscription: %+v", second.TLS)
	}
	if first.Generation != second.Generation {
		t.Fatalf("server-only TLS metadata changed subscription generation: %s != %s", first.Generation, second.Generation)
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

func TestPingFailsClosedWhenRuntimeIsUnavailable(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		LoadRuntime: func() (Runtime, error) { return Runtime{}, os.ErrNotExist },
	})
	req := httptest.NewRequest(http.MethodPost, PathPing, bytes.NewBufferString(`{"nonce":"n1"}`))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("ping status = %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestPingFailsClosedWhenRuntimeAuthIsIncomplete(t *testing.T) {
	for _, users := range [][]AuthUser{nil, {{Label: "alice"}}, {{Credential: "secret"}}} {
		handler := NewHandler(HandlerOptions{
			LoadRuntime: func() (Runtime, error) { return Runtime{AuthUsers: users}, nil },
		})
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, PathPing, bytes.NewBufferString(`{"nonce":"n1"}`)))
		if resp.Code != http.StatusServiceUnavailable {
			t.Fatalf("users=%v ping status=%d body=%s", users, resp.Code, resp.Body.String())
		}
	}
}

func TestProtectedPingRejectsMissingAndInvalidSignatures(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	body := []byte(`{"nonce":"n1"}`)
	handler := NewHandler(HandlerOptions{
		LoadRuntime: func() (Runtime, error) {
			return Runtime{AuthUsers: []AuthUser{{Label: "alice", Credential: "secret"}}}, nil
		},
		Now: func() time.Time { return now },
	})

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodPost, PathPing, bytes.NewReader(body)))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing signature status=%d body=%s", missing.Code, missing.Body.String())
	}

	invalidReq := signedRequest(t, http.MethodPost, PathPing, body, now)
	invalidReq.Header.Set(HeaderSignature, "00")
	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, invalidReq)
	if invalid.Code != http.StatusForbidden {
		t.Fatalf("invalid signature status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

func TestDiagnosticsHandlerServesOnlyPublicReadinessAndPing(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	handler := NewDiagnosticsHandler(DiagnosticsOptions{Now: func() time.Time { return now }})
	body := []byte(`{"nonce":"n1"}`)

	for _, signed := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodPost, PathPing, bytes.NewReader(body))
		if signed {
			if err := ApplyHeaders(req, "unknown", "unknown-secret", "auth-nonce", body, now); err != nil {
				t.Fatal(err)
			}
		}
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("signed=%v ping status=%d body=%s", signed, resp.Code, resp.Body.String())
		}
		var pong PingResponse
		if err := json.NewDecoder(resp.Body).Decode(&pong); err != nil || pong.Nonce != "n1" {
			t.Fatalf("signed=%v pong=%+v err=%v", signed, pong, err)
		}
	}

	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, PathReady, nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("ready status=%d", ready.Code)
	}
	for _, path := range []string{PathHeartbeat, PathSubscription, PathCredentialsRotate, PathCredentialsAck, "/control/v1/ha/status"} {
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if resp.Code != http.StatusNotFound {
			t.Fatalf("GET %s status=%d", path, resp.Code)
		}
	}
}

func TestPingProtocolContractAcrossCompositions(t *testing.T) {
	protected := NewHandler(HandlerOptions{LoadRuntime: func() (Runtime, error) {
		return Runtime{AuthUsers: []AuthUser{{Label: "alice", Credential: "secret"}}}, nil
	}})
	public := NewDiagnosticsHandler(DiagnosticsOptions{})
	for name, handler := range map[string]http.Handler{"protected": protected, "public": public} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				method string
				body   string
				want   int
			}{{http.MethodGet, "", http.StatusMethodNotAllowed}, {http.MethodPost, "{", http.StatusBadRequest}, {http.MethodPost, `{}`, http.StatusUnauthorized}} {
				want := tc.want
				if name == "public" && tc.body == `{}` {
					want = http.StatusBadRequest
				}
				resp := httptest.NewRecorder()
				handler.ServeHTTP(resp, httptest.NewRequest(tc.method, PathPing, bytes.NewBufferString(tc.body)))
				if resp.Code != want {
					t.Fatalf("%s %q status=%d want=%d body=%s", tc.method, tc.body, resp.Code, want, resp.Body.String())
				}
			}
		})
	}
}

func TestRotationChallengeAndProofDoNotExposeCredential(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	runtime := Runtime{Subscription: Subscription{Generation: "sub-1"}, RotationUsers: []RotationUser{{UserLabel: "alice", ActiveCredential: "new", PreviousCredentialForRotation: "old", RotationExpiresAt: now.Add(time.Hour), CredentialGeneration: 2}}}
	h := NewHandler(HandlerOptions{LoadRuntime: func() (Runtime, error) { return runtime, nil }, Now: func() time.Time { return now }})
	challengeBody := []byte(`{"user_label":"alice","action":"challenge"}`)
	challengeReq := httptest.NewRequest(http.MethodPost, PathCredentialsRotate, bytes.NewReader(challengeBody))
	challengeResp := httptest.NewRecorder()
	h.ServeHTTP(challengeResp, challengeReq)
	if challengeResp.Code != http.StatusOK {
		t.Fatalf("challenge status=%d", challengeResp.Code)
	}
	var challenge RotationChallenge
	_ = json.NewDecoder(challengeResp.Body).Decode(&challenge)
	body, _ := json.Marshal(RotationRequest{UserLabel: "alice", Nonce: challenge.Nonce, Proof: RotationProof("old", challenge.Nonce)})
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, PathCredentialsRotate, bytes.NewReader(body)))
	if resp.Code != http.StatusOK {
		t.Fatalf("rotation status=%d body=%s", resp.Code, resp.Body.String())
	}
	var result RotationResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if !result.RotationPending || result.ActiveCredential != "new" || result.CredentialGeneration != 2 {
		t.Fatalf("unexpected rotation response: %#v", result)
	}
	bad := httptest.NewRecorder()
	h.ServeHTTP(bad, httptest.NewRequest(http.MethodPost, PathCredentialsRotate, bytes.NewReader(body)))
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("replayed proof status=%d", bad.Code)
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
