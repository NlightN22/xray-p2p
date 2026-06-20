package controlplane

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
)

type RuntimeLoader func() (Runtime, error)

type HandlerOptions struct {
	LoadRuntime RuntimeLoader
	Heartbeat   *heartbeat.Store
	Now         func() time.Time
	AuthWindow  time.Duration
}

func NewHandler(opts HandlerOptions) http.Handler {
	h := &handler{
		load:       opts.LoadRuntime,
		heartbeat:  opts.Heartbeat,
		now:        opts.Now,
		authWindow: opts.AuthWindow,
	}
	if h.now == nil {
		h.now = time.Now
	}
	mux := http.NewServeMux()
	mux.HandleFunc(PathReady, h.ready)
	mux.HandleFunc(PathPing, h.ping)
	mux.HandleFunc(PathHeartbeat, h.heartbeatPost)
	mux.HandleFunc(PathSubscription, h.subscription)
	return mux
}

type handler struct {
	load       RuntimeLoader
	heartbeat  *heartbeat.Store
	now        func() time.Time
	authWindow time.Duration
	mu         sync.Mutex
}

func (h *handler) ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ready": true})
}

func (h *handler) ping(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req PingRequest
	rt, body, ok := h.loadAndAuth(w, r, &req)
	if !ok {
		return
	}
	_ = rt
	if req.Nonce == "" {
		writeError(w, http.StatusBadRequest, "nonce is required")
		return
	}
	writeJSON(w, http.StatusOK, PingResponse{Nonce: req.Nonce, ServerAt: h.now().UTC()})
	_ = body
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
	if h.heartbeat != nil {
		if _, err := h.heartbeat.Update(payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
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
	rt, _, ok := h.loadAndAuth(w, r, nil)
	if !ok {
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
	writeJSON(w, http.StatusOK, sub)
}

func (h *handler) loadAndAuth(w http.ResponseWriter, r *http.Request, dst any) (Runtime, []byte, bool) {
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
