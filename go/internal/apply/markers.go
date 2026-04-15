package apply

import (
	"fmt"
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

