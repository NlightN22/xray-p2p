package clientcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func waitForServiceState(ctx context.Context, ctrl servicecontrol.Controller, role servicecontrol.Role, desired string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		status, err := ctrl.Status(ctx, role)
		if err != nil {
			if errors.Is(err, servicecontrol.ErrUnsupported) {
				return err
			}
			return err
		}
		if serviceStateMatches(status.State, desired) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("service %s did not reach %s (state=%s)", role, desired, status.State)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func serviceStateMatches(state, desired string) bool {
	state = strings.ToLower(strings.TrimSpace(state))
	desired = strings.ToLower(strings.TrimSpace(desired))
	switch desired {
	case "stopped":
		return state == "inactive" || state == "failed" || state == "dead" || state == "stopped"
	case "running":
		return state == "active" || state == "running"
	default:
		return state == desired
	}
}
