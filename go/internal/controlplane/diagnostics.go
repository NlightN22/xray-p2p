package controlplane

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

type PingAuthProvider func() ([]AuthUser, error)

type DiagnosticsOptions struct {
	Now        func() time.Time
	AuthWindow time.Duration
}

func NewDiagnosticsHandler(opts DiagnosticsOptions) http.Handler {
	h := newHandler(HandlerOptions{
		PingAuth:   func() ([]AuthUser, error) { return []AuthUser{}, nil },
		Now:        opts.Now,
		AuthWindow: opts.AuthWindow,
	})
	mux := http.NewServeMux()
	mux.HandleFunc(PathReady, diagnosticsReady)
	mux.HandleFunc(PathPing, h.ping)
	return mux
}

func diagnosticsReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ready":        true,
		"capabilities": []string{"xp2p-diag"},
	})
}

func registerDiagnosticsRoutes(mux *http.ServeMux, h *handler) {
	mux.HandleFunc(PathReady, h.ready)
	mux.HandleFunc(PathPing, h.ping)
}

func validatePingAuthUsers(users []AuthUser) error {
	if len(users) == 0 {
		return errors.New("control ping auth metadata is missing")
	}
	for _, user := range users {
		if strings.TrimSpace(user.Label) == "" || strings.TrimSpace(user.Credential) == "" {
			return errors.New("control ping auth metadata is incomplete")
		}
	}
	return nil
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
	users, err := h.pingAuth()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "control unavailable")
		return
	}
	var req PingRequest
	body, err := readBody(r, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := VerifyRequest(r, body, users, h.now(), h.authWindow); err != nil {
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
