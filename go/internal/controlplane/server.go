package controlplane

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/ha"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
)

type RuntimeLoader func() (Runtime, error)

type HandlerOptions struct {
	LoadRuntime RuntimeLoader
	Heartbeat   *heartbeat.Store
	Now         func() time.Time
	AuthWindow  time.Duration
	Acknowledge func(userLabel string, credentialGeneration int) error
	HAStore     *ha.Store
	ReloadHA    func(*ha.Store) error
}

func NewHandler(opts HandlerOptions) http.Handler {
	h := &handler{
		load:        opts.LoadRuntime,
		heartbeat:   opts.Heartbeat,
		now:         opts.Now,
		authWindow:  opts.AuthWindow,
		acknowledge: opts.Acknowledge,
	}
	if h.now == nil {
		h.now = time.Now
	}
	mux := http.NewServeMux()
	mux.HandleFunc(PathReady, h.ready)
	mux.HandleFunc(PathPing, h.ping)
	mux.HandleFunc(PathHeartbeat, h.heartbeatPost)
	mux.HandleFunc(PathSubscription, h.subscription)
	mux.HandleFunc(PathCredentialsRotate, h.rotate)
	mux.HandleFunc(PathCredentialsAck, h.ack)
	if opts.HAStore != nil {
		var haHandler http.Handler = ha.NewHTTPHandler(opts.HAStore)
		if opts.ReloadHA != nil {
			haHandler = reloadHAHandler(opts.HAStore, opts.ReloadHA, haHandler)
		}
		mux.Handle("/control/v1/ha/", haHandler)
	}
	return mux
}

func reloadHAHandler(store *ha.Store, reload func(*ha.Store) error, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := reload(store); err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		next.ServeHTTP(w, r)
	})
}

type handler struct {
	load        RuntimeLoader
	heartbeat   *heartbeat.Store
	now         func() time.Time
	authWindow  time.Duration
	acknowledge func(string, int) error
	mu          sync.Mutex
	challenges  map[string]rotationChallenge
}

type rotationChallenge struct {
	nonce     string
	expiresAt time.Time
}

func (h *handler) ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ready": true})
}

func (h *handler) rotate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req RotationRequest
	rt, _, ok := h.loadRotationRequest(w, r, &req)
	if !ok {
		return
	}
	if req.Action == "challenge" {
		if rotationUser(rt.RotationUsers, req.UserLabel) == nil {
			h.rotationAuthFailure(w)
			return
		}
		nonce, err := h.newChallenge(req.UserLabel)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "challenge unavailable")
			return
		}
		writeJSON(w, http.StatusOK, nonce)
		return
	}
	user := h.verifyRotation(rt, req)
	if user == nil {
		h.rotationAuthFailure(w)
		return
	}
	if h.now().After(user.RotationExpiresAt) {
		user.PreviousCredentialForRotation = ""
	}
	if user.PreviousCredentialForRotation == "" || !sameProof(req.Proof, user.PreviousCredentialForRotation, req.Nonce) {
		writeJSON(w, http.StatusOK, RotationResponse{RotationPending: false, CredentialGeneration: user.CredentialGeneration, SubscriptionGeneration: rt.Subscription.Generation})
		return
	}
	writeJSON(w, http.StatusOK, RotationResponse{RotationPending: true, ActiveCredential: user.ActiveCredential, CredentialGeneration: user.CredentialGeneration, SubscriptionGeneration: rt.Subscription.Generation})
}

func (h *handler) ack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req RotationRequest
	rt, _, ok := h.loadRotationRequest(w, r, &req)
	if !ok {
		return
	}
	user := h.verifyRotation(rt, req)
	if user == nil || !sameProof(req.Proof, user.ActiveCredential, req.Nonce) || user.PreviousCredentialForRotation == "" {
		h.rotationAuthFailure(w)
		return
	}
	if h.acknowledge == nil || h.acknowledge(user.UserLabel, user.CredentialGeneration) != nil {
		writeError(w, http.StatusServiceUnavailable, "rotation acknowledgement unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *handler) loadRotationRequest(w http.ResponseWriter, r *http.Request, req *RotationRequest) (Runtime, []byte, bool) {
	rt, err := h.runtime()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "control unavailable")
		return Runtime{}, nil, false
	}
	body, err := readBody(r, req)
	if err != nil {
		h.rotationAuthFailure(w)
		return Runtime{}, nil, false
	}
	return rt, body, true
}

func (h *handler) newChallenge(label string) (RotationChallenge, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return RotationChallenge{}, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.challenges == nil {
		h.challenges = make(map[string]rotationChallenge)
	}
	now := h.now().UTC()
	c := rotationChallenge{nonce: hex.EncodeToString(raw), expiresAt: now.Add(2 * time.Minute)}
	h.challenges[label] = c
	return RotationChallenge{Nonce: c.nonce, ExpiresAt: c.expiresAt}, nil
}

func (h *handler) verifyRotation(rt Runtime, req RotationRequest) *RotationUser {
	user := rotationUser(rt.RotationUsers, req.UserLabel)
	if user == nil || req.Nonce == "" || req.Proof == "" {
		return nil
	}
	h.mu.Lock()
	c, ok := h.challenges[req.UserLabel]
	if ok && (c.nonce != req.Nonce || h.now().After(c.expiresAt)) {
		ok = false
	}
	if ok {
		delete(h.challenges, req.UserLabel)
	}
	h.mu.Unlock()
	if !ok || (!sameProof(req.Proof, user.ActiveCredential, req.Nonce) && (user.PreviousCredentialForRotation == "" || h.now().After(user.RotationExpiresAt) || !sameProof(req.Proof, user.PreviousCredentialForRotation, req.Nonce))) {
		return nil
	}
	return user
}

func rotationUser(users []RotationUser, label string) *RotationUser {
	for i := range users {
		if strings.EqualFold(users[i].UserLabel, label) {
			return &users[i]
		}
	}
	return nil
}
func sameProof(proof, credential, nonce string) bool {
	got, err := hex.DecodeString(proof)
	if err != nil {
		return false
	}
	want, _ := hex.DecodeString(RotationProof(credential, nonce))
	return hmac.Equal(got, want)
}
func (h *handler) rotationAuthFailure(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "authentication failed")
}

func (h *handler) ping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req PingRequest
	body, err := readBody(r, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rt, err := h.runtime()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "control unavailable")
		return
	}
	if err := VerifyRequest(r, body, rt.AuthUsers, h.now(), h.authWindow); err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, ErrAuthInvalid) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}
	if req.Nonce == "" {
		writeError(w, http.StatusBadRequest, "nonce is required")
		return
	}
	writeJSON(w, http.StatusOK, PingResponse{Nonce: req.Nonce, ServerAt: h.now().UTC()})
}

func (h *handler) heartbeatPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var payload heartbeat.Payload
	_, _, ok := h.loadAndAuth(w, r, &payload)
	if !ok {
		return
	}
	payload.Timestamp = time.Time{}
	healthy := true
	payload.Healthy = &healthy
	payload.Mode = heartbeat.ModeRequired
	if h.heartbeat != nil {
		if _, err := h.heartbeat.Update(payload); err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *handler) subscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rt, body, ok := h.loadRequest(w, r, nil)
	if !ok {
		return
	}
	if err := VerifyRequest(r, body, subscriptionAuthUsers(rt), h.now(), h.authWindow); err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, ErrAuthInvalid) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}
	sub := rt.Subscription
	if sub.IssuedAt.IsZero() {
		rebuilt, err := BuildSubscription(sub, h.now(), time.Hour)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sub = rebuilt
	}
	if known := strings.TrimSpace(r.Header.Get(HeaderKnownGeneration)); known != "" && known == sub.Generation {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

func (h *handler) loadAndAuth(w http.ResponseWriter, r *http.Request, dst any) (Runtime, []byte, bool) {
	rt, body, ok := h.loadRequest(w, r, dst)
	if !ok {
		return Runtime{}, nil, false
	}
	if err := VerifyRequest(r, body, rt.AuthUsers, h.now(), h.authWindow); err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, ErrAuthInvalid) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return Runtime{}, nil, false
	}
	return rt, body, true
}

func (h *handler) loadRequest(w http.ResponseWriter, r *http.Request, dst any) (Runtime, []byte, bool) {
	rt, err := h.runtime()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return Runtime{}, nil, false
	}
	body, err := readBody(r, dst)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return Runtime{}, nil, false
	}
	return rt, body, true
}

func subscriptionAuthUsers(rt Runtime) []AuthUser {
	active := make(map[string]string, len(rt.RotationUsers))
	for _, user := range rt.RotationUsers {
		label := strings.TrimSpace(user.UserLabel)
		credential := strings.TrimSpace(user.ActiveCredential)
		if label != "" && credential != "" {
			active[strings.ToLower(label)] = credential
		}
	}
	if len(active) == 0 {
		return rt.AuthUsers
	}
	out := make([]AuthUser, 0, len(rt.AuthUsers))
	for _, user := range rt.AuthUsers {
		label := strings.TrimSpace(user.Label)
		if label == "" {
			continue
		}
		if credential, ok := active[strings.ToLower(label)]; ok {
			if strings.TrimSpace(user.Credential) == credential {
				out = append(out, user)
			}
			continue
		}
		out = append(out, user)
	}
	return out
}

func (h *handler) runtime() (Runtime, error) {
	if h.load == nil {
		return Runtime{}, errors.New("control runtime loader is not configured")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.load()
}

func readBody(r *http.Request, dst any) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	if dst == nil || len(bytes.TrimSpace(body)) == 0 {
		return body, nil
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return nil, fmt.Errorf("decode request body: %w", err)
	}
	return body, nil
}

func LoadRuntimeFile(path string) (Runtime, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Runtime{}, fmt.Errorf("read runtime metadata: %w", err)
	}
	var doc struct {
		Control Runtime `json:"control"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Runtime{}, fmt.Errorf("parse runtime metadata: %w", err)
	}
	if doc.Control.Subscription.Generation == "" {
		return Runtime{}, errors.New("control subscription metadata is missing")
	}
	return doc.Control, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
