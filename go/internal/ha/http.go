package ha

import (
	"encoding/json"
	"net/http"
)

const (
	PathPrepare     = "/control/v1/ha/prepare"
	PathAcknowledge = "/control/v1/ha/acknowledge"
	PathCommit      = "/control/v1/ha/commit"
	PathStatus      = "/control/v1/ha/status"
)

// NewHTTPHandler is mounted behind the server's HTTPS listener. Trusted peers
// use normal certificate-chain and hostname verification; this handler adds
// explicit peer membership and a generation-bound HMAC.
func NewHTTPHandler(store *Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(PathPrepare, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request PrepareRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid HA prepare request", http.StatusBadRequest)
			return
		}
		ack := store.Prepare(request)
		if !ack.Ready {
			w.WriteHeader(http.StatusForbidden)
		}
		_ = json.NewEncoder(w).Encode(ack)
	})
	mux.HandleFunc(PathAcknowledge, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var ack Acknowledgement
		if err := json.NewDecoder(r.Body).Decode(&ack); err != nil {
			http.Error(w, "invalid HA acknowledgement", http.StatusBadRequest)
			return
		}
		if err := store.Acknowledge(ack); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(PathCommit, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request CommitRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid HA commit request", http.StatusBadRequest)
			return
		}
		generation, err := store.CommitAuthorized(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		_ = json.NewEncoder(w).Encode(generation)
	})
	mux.HandleFunc(PathStatus, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		committed, pending, acks := store.Status()
		_ = json.NewEncoder(w).Encode(struct {
			Committed        Generation        `json:"committed"`
			Pending          *Generation       `json:"pending,omitempty"`
			Acknowledgements []Acknowledgement `json:"acknowledgements"`
			Recovery         RecoveryState     `json:"recovery"`
		}{committed, pending, acks, store.RecoveryState()})
	})
	return mux
}
