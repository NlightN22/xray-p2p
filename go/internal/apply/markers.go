package apply

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// RemoveRoleMarkers removes apply.request and apply.error if they belong to the supplied role.
// When apply.request targets RoleAny it is preserved because it may still be needed by the other role.
func RemoveRoleMarkers(requestPath, errorPath, role string) error {
	normalized := strings.TrimSpace(strings.ToLower(role))
	if normalized == "" {
		return fmt.Errorf("apply: remove markers: role is required")
	}

	if req, exists, err := ReadRequest(requestPath); err == nil {
		if exists {
			if req.Role == "" || strings.EqualFold(req.Role, normalized) {
				if err := RemoveRequest(requestPath); err != nil {
					return err
				}
			}
		}
	} else {
		if err := RemoveRequest(requestPath); err != nil {
			return fmt.Errorf("apply: remove request after read failure: %w", err)
		}
	}

	if marker, exists, err := ReadError(errorPath); err == nil {
		if exists {
			if marker.Role == "" || strings.EqualFold(marker.Role, normalized) {
				if err := RemoveError(errorPath); err != nil {
					return err
				}
			}
		}
	} else {
		if err := RemoveError(errorPath); err != nil {
			return fmt.Errorf("apply: remove error marker after read failure: %w", err)
		}
	}

	return nil
}

// CompleteRequest removes markers only when they still belong to req.
// A newer request is preserved so an older apply cannot acknowledge work it
// did not compile.
func CompleteRequest(requestPath, errorPath string, req Request) error {
	if strings.TrimSpace(req.ID) == "" {
		return nil
	}
	claimedPath := requestPath + ".complete-" + req.ID
	_ = os.Remove(claimedPath)
	if err := os.Rename(requestPath, claimedPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("apply: claim request completion: %w", err)
	}
	claimed, exists, err := ReadRequest(claimedPath)
	if err != nil || !exists || claimed.ID != req.ID {
		if _, statErr := os.Stat(requestPath); errors.Is(statErr, os.ErrNotExist) {
			_ = os.Rename(claimedPath, requestPath)
		} else {
			_ = os.Remove(claimedPath)
		}
		return err
	}
	if claimed.Role == RoleAny {
		if _, statErr := os.Stat(requestPath); errors.Is(statErr, os.ErrNotExist) {
			_ = os.Rename(claimedPath, requestPath)
		} else {
			_ = os.Remove(claimedPath)
		}
		return nil
	}
	_ = os.Remove(claimedPath)
	marker, markerExists, err := ReadError(errorPath)
	if err != nil {
		return err
	}
	if markerExists && marker.RequestID == req.ID {
		return RemoveError(errorPath)
	}
	return nil
}
