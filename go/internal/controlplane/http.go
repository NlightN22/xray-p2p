package controlplane

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

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
