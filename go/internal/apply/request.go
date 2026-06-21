package apply

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/identity"
)

type Request struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Role      string    `json:"role"`
}

func NewRequest(role string) (Request, error) {
	id, err := identity.NewRequestID()
	if err != nil {
		return Request{}, fmt.Errorf("apply: generate id: %w", err)
	}
	return Request{
		ID:        id,
		Timestamp: time.Now().UTC(),
		Role:      strings.TrimSpace(strings.ToLower(role)),
	}, nil
}

func (r Request) MatchesRole(role string) bool {
	want := strings.TrimSpace(strings.ToLower(role))
	if want == "" {
		return false
	}
	if r.Role == "" {
		return true
	}
	if strings.EqualFold(r.Role, want) {
		return true
	}
	if strings.EqualFold(r.Role, RoleAny) {
		return true
	}
	return false
}

func WriteRequest(path string, req Request, auditPath string) error {
	if existing, exists, err := ReadRequest(path); err == nil {
		if exists && existing.ID != "" {
			if existing.MatchesRole(req.Role) {
				return nil
			}
			if existing.Role != "" && req.Role != "" && !strings.EqualFold(existing.Role, RoleAny) && !strings.EqualFold(req.Role, RoleAny) {
				req.Role = RoleAny
			}
		}
	}
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return fmt.Errorf("apply: encode request: %w", err)
	}
	data = append(data, '\n')
	return configio.WriteBytes(path, data, configio.WriteOptions{
		AuditPath:         auditPath,
		IgnoreAuditErrors: true,
	})
}

func ReadRequest(path string) (Request, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Request{}, false, nil
		}
		return Request{}, false, fmt.Errorf("apply: read request: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return Request{}, true, nil
	}
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		trimmed := strings.TrimSpace(string(data))
		repaired := strings.ReplaceAll(trimmed, "\\r\\n", "\n")
		repaired = strings.ReplaceAll(repaired, "\\n", "\n")
		if retryErr := json.Unmarshal([]byte(repaired), &req); retryErr != nil {
			return Request{}, true, fmt.Errorf("apply: parse request: %w", err)
		}
	}
	req.Role = strings.TrimSpace(strings.ToLower(req.Role))
	return req, true, nil
}

func RemoveRequest(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("apply: remove request: %w", err)
	}
	return nil
}
