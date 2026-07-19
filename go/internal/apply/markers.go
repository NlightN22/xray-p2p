package apply

import (
	"fmt"
	"strings"
)

// RemoveRoleMarkers removes only the supplied role generation and error.
func RemoveRoleMarkers(requestPath, errorPath, role string) error {
	role = normalizeRole(role)
	if role != RoleClient && role != RoleServer {
		return fmt.Errorf("apply: remove markers: concrete role is required")
	}
	if err := removeRoleRequest(requestPath, role); err != nil {
		return err
	}
	marker, exists, err := ReadError(errorPath)
	if err != nil {
		return RemoveError(errorPath)
	}
	if exists && (marker.Role == "" || strings.EqualFold(marker.Role, role)) {
		return RemoveError(errorPath)
	}
	return nil
}

// CompleteRequest removes only the exact role generation that was applied. A
// newer generation and the other role generation are preserved.
func CompleteRequest(requestPath, errorPath string, req Request) error {
	role := normalizeRole(req.Role)
	if strings.TrimSpace(req.ID) == "" || (role != RoleClient && role != RoleServer) {
		return nil
	}
	removed := false
	if err := withRequestLock(requestPath, func() error {
		doc, legacy, exists, err := readRequestDocument(requestPath)
		if err != nil || !exists {
			return err
		}
		if legacy != nil {
			return completeLegacyRequest(requestPath, &removed, *legacy, req)
		}
		current, pending := doc.Requests[role]
		if !pending || current.ID != req.ID {
			return nil
		}
		delete(doc.Requests, role)
		removed = true
		return persistRemainingRequests(requestPath, doc)
	}); err != nil {
		return err
	}
	if !removed {
		return nil
	}
	marker, exists, err := ReadError(errorPath)
	if err != nil {
		return err
	}
	if exists && marker.RequestID == req.ID {
		return RemoveError(errorPath)
	}
	return nil
}

func removeRoleRequest(requestPath, role string) error {
	return withRequestLock(requestPath, func() error {
		doc, legacy, exists, err := readRequestDocument(requestPath)
		if err != nil || !exists {
			return err
		}
		if legacy != nil {
			legacyRole := normalizeRole(legacy.Role)
			if legacyRole != "" && legacyRole != RoleAny && legacyRole != role {
				return nil
			}
			if legacyRole == role {
				return RemoveRequest(requestPath)
			}
			other, err := requestForRole(*legacy, otherRequestRole(role))
			if err != nil {
				return err
			}
			doc.Requests[other.Role] = other
			return writeRequestDocument(requestPath, doc, "")
		}
		delete(doc.Requests, role)
		return persistRemainingRequests(requestPath, doc)
	})
}

func completeLegacyRequest(requestPath string, removed *bool, legacy, completed Request) error {
	legacyRole := normalizeRole(legacy.Role)
	if legacy.ID != completed.ID || (legacyRole != "" && legacyRole != RoleAny && legacyRole != completed.Role) {
		return nil
	}
	*removed = true
	if legacyRole == completed.Role {
		return RemoveRequest(requestPath)
	}
	remaining, err := NewRequest(otherRequestRole(completed.Role))
	if err != nil {
		return err
	}
	if !legacy.Timestamp.IsZero() {
		remaining.Timestamp = legacy.Timestamp
	}
	doc := requestDocument{
		Version:  requestDocumentVersion,
		Requests: map[string]Request{remaining.Role: remaining},
	}
	return writeRequestDocument(requestPath, doc, "")
}

func persistRemainingRequests(path string, doc requestDocument) error {
	if len(doc.Requests) == 0 {
		return RemoveRequest(path)
	}
	return writeRequestDocument(path, doc, "")
}
