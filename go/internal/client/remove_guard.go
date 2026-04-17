package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func ensureServiceInactive(ctx context.Context, role control.Role, stopHint string) error {
	status, err := control.Default().Status(ctx, role)
	if err != nil {
		if errors.Is(err, control.ErrUnsupported) {
			return fmt.Errorf("unable to determine service status before removal: %w", err)
		}
		return fmt.Errorf("failed to determine service status before removal: %w", err)
	}
	if status.Active {
		return fmt.Errorf("%s service is running; stop it first (%s)", role, stopHint)
	}
	if !serviceStateInactive(status.State) {
		return fmt.Errorf("%s service is not stopped (state: %s); stop it first (%s)", role, strings.TrimSpace(status.State), stopHint)
	}
	return nil
}

func serviceStateInactive(state string) bool {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "", "inactive", "stopped", "stopped.", "unknown", "failed":
		return true
	default:
		return false
	}
}
