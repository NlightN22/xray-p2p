//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/ports"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func (m *TunnelManager) WaitServiceRestart(ctx context.Context, req ports.ServiceWaitRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	timeout := parseDuration(req.Timeout, 90*time.Second)
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := waitForApplyRequestClear(waitCtx); err != nil {
		return err
	}

	role, err := mapServiceRole(req.Role)
	if err != nil {
		return err
	}
	ctrl := servicecontrol.Default()
	if err := waitForServiceActive(waitCtx, ctrl, role); err != nil {
		return err
	}
	return nil
}

func waitForApplyRequestClear(ctx context.Context) error {
	path := config.ApplyRequestPath()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("read apply request: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitForServiceActive(ctx context.Context, ctrl servicecontrol.Controller, role servicecontrol.Role) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		status, err := ctrl.Status(ctx, role)
		if err != nil {
			if errors.Is(err, servicecontrol.ErrUnsupported) {
				return nil
			}
			return err
		}
		if status.Active {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func mapServiceRole(role string) (servicecontrol.Role, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "client":
		return servicecontrol.RoleClient, nil
	case "server":
		return servicecontrol.RoleServer, nil
	default:
		return "", fmt.Errorf("unsupported role %q", role)
	}
}
