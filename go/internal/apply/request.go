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

const requestDocumentVersion = 2

type Request struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Role      string    `json:"role"`
}

type requestDocument struct {
	Version  int                `json:"version"`
	Requests map[string]Request `json:"requests"`
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
	want := normalizeRole(role)
	if want == "" {
		return false
	}
	return r.Role == "" || strings.EqualFold(r.Role, want) || strings.EqualFold(r.Role, RoleAny)
}

// WriteRequest replaces only the requested role generation. RoleAny queues
// independent generations for both roles.
func WriteRequest(path string, req Request, auditPath string) error {
	return withRequestLock(path, func() error {
		doc, legacy, _, err := readRequestDocument(path)
		if err != nil {
			return err
		}
		if legacy != nil {
			if err := mergeLegacyRequest(&doc, *legacy, normalizeRole(req.Role)); err != nil {
				return err
			}
		}
		role := normalizeRole(req.Role)
		switch role {
		case RoleClient, RoleServer:
			req.Role = role
			doc.Requests[role] = req
		case "", RoleAny:
			clientReq, err := requestForRole(req, RoleClient)
			if err != nil {
				return err
			}
			serverReq, err := NewRequest(RoleServer)
			if err != nil {
				return err
			}
			if !req.Timestamp.IsZero() {
				serverReq.Timestamp = req.Timestamp
			}
			doc.Requests[RoleClient] = clientReq
			doc.Requests[RoleServer] = serverReq
		default:
			return fmt.Errorf("apply: invalid request role %q", req.Role)
		}
		return writeRequestDocument(path, doc, auditPath)
	})
}

// ReadRequestForRole returns the exact pending generation for one role. A
// legacy RoleAny marker is migrated before it is returned.
func ReadRequestForRole(path, role string) (Request, bool, error) {
	role = normalizeRole(role)
	if role != RoleClient && role != RoleServer {
		return Request{}, false, fmt.Errorf("apply: concrete request role is required")
	}
	var result Request
	var exists bool
	err := withRequestLock(path, func() error {
		doc, legacy, fileExists, err := readRequestDocument(path)
		if err != nil || !fileExists {
			return err
		}
		if legacy == nil {
			result, exists = doc.Requests[role]
			return nil
		}
		if !legacy.MatchesRole(role) {
			return nil
		}
		if normalizeRole(legacy.Role) == role {
			result, exists = *legacy, true
			result.Role = role
			return nil
		}
		if err := migrateLegacyAny(&doc, *legacy, role); err != nil {
			return err
		}
		if err := writeRequestDocument(path, doc, ""); err != nil {
			return err
		}
		result, exists = doc.Requests[role]
		return nil
	})
	return result, exists, err
}

// ReadRequest preserves the legacy inspection API. Multi-role documents are
// reported as RoleAny; role-aware consumers must use ReadRequestForRole.
func ReadRequest(path string) (Request, bool, error) {
	doc, legacy, exists, err := readRequestDocument(path)
	if err != nil || !exists {
		return Request{}, exists, err
	}
	if legacy != nil {
		return *legacy, true, nil
	}
	if len(doc.Requests) == 1 {
		for _, req := range doc.Requests {
			return req, true, nil
		}
	}
	if len(doc.Requests) > 1 {
		return Request{Role: RoleAny}, true, nil
	}
	return Request{}, false, nil
}

func RemoveRequest(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("apply: remove request: %w", err)
	}
	return nil
}

func readRequestDocument(path string) (requestDocument, *Request, bool, error) {
	doc := requestDocument{Version: requestDocumentVersion, Requests: map[string]Request{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return doc, nil, false, nil
		}
		return doc, nil, false, fmt.Errorf("apply: read request: %w", err)
	}
	data = repairRequestJSON(data)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err == nil && fields["version"] != nil {
		if err := json.Unmarshal(data, &doc); err != nil {
			return doc, nil, true, fmt.Errorf("apply: parse request document: %w", err)
		}
		if doc.Version != requestDocumentVersion || doc.Requests == nil {
			return doc, nil, true, fmt.Errorf("apply: unsupported request document version %d", doc.Version)
		}
		normalizeRequestDocument(&doc)
		return doc, nil, true, nil
	}
	var legacy Request
	if len(strings.TrimSpace(string(data))) != 0 {
		if err := json.Unmarshal(data, &legacy); err != nil {
			return doc, nil, true, fmt.Errorf("apply: parse request: %w", err)
		}
	}
	legacy.Role = normalizeRole(legacy.Role)
	return doc, &legacy, true, nil
}

func writeRequestDocument(path string, doc requestDocument, auditPath string) error {
	doc.Version = requestDocumentVersion
	if doc.Requests == nil {
		doc.Requests = map[string]Request{}
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("apply: encode request: %w", err)
	}
	return configio.WriteBytes(path, append(data, '\n'), configio.WriteOptions{
		AuditPath:         auditPath,
		IgnoreAuditErrors: true,
	})
}

func repairRequestJSON(data []byte) []byte {
	trimmed := strings.TrimSpace(string(data))
	repaired := strings.ReplaceAll(trimmed, "\\r\\n", "\n")
	return []byte(strings.ReplaceAll(repaired, "\\n", "\n"))
}

func normalizeRequestDocument(doc *requestDocument) {
	normalized := make(map[string]Request, len(doc.Requests))
	for key, req := range doc.Requests {
		role := normalizeRole(key)
		if role != RoleClient && role != RoleServer {
			continue
		}
		req.Role = role
		normalized[role] = req
	}
	doc.Requests = normalized
}

func mergeLegacyRequest(doc *requestDocument, legacy Request, replacingRole string) error {
	role := normalizeRole(legacy.Role)
	if role == RoleClient || role == RoleServer {
		legacy.Role = role
		doc.Requests[role] = legacy
		return nil
	}
	if replacingRole == RoleClient || replacingRole == RoleServer {
		other := otherRequestRole(replacingRole)
		req, err := requestForRole(legacy, other)
		if err != nil {
			return err
		}
		doc.Requests[other] = req
		return nil
	}
	return nil
}

func migrateLegacyAny(doc *requestDocument, legacy Request, requestedRole string) error {
	requested, err := requestForRole(legacy, requestedRole)
	if err != nil {
		return err
	}
	other, err := NewRequest(otherRequestRole(requestedRole))
	if err != nil {
		return err
	}
	if !legacy.Timestamp.IsZero() {
		other.Timestamp = legacy.Timestamp
	}
	doc.Requests[requestedRole] = requested
	doc.Requests[other.Role] = other
	return nil
}

func requestForRole(req Request, role string) (Request, error) {
	req.Role = role
	if strings.TrimSpace(req.ID) != "" {
		return req, nil
	}
	created, err := NewRequest(role)
	if err != nil {
		return Request{}, err
	}
	if !req.Timestamp.IsZero() {
		created.Timestamp = req.Timestamp
	}
	return created, nil
}

func normalizeRole(role string) string {
	return strings.TrimSpace(strings.ToLower(role))
}

func otherRequestRole(role string) string {
	if role == RoleClient {
		return RoleServer
	}
	return RoleClient
}
