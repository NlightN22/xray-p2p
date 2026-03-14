//go:build windows

package control

import (
	"context"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/platform/windows/scctl"
	"golang.org/x/sys/windows/svc"
)

type windowsController struct{}

func defaultController() Controller {
	return windowsController{}
}

func (windowsController) Start(ctx context.Context, role Role) error {
	name, err := serviceName(role)
	if err != nil {
		return err
	}
	return scctl.Run(ctx, "start", name, scctl.AllowServiceAlreadyRunning)
}

func (windowsController) Stop(ctx context.Context, role Role) error {
	name, err := serviceName(role)
	if err != nil {
		return err
	}
	return scctl.Run(ctx, "stop", name, scctl.AllowServiceNotStarted)
}

func (windowsController) Status(ctx context.Context, role Role) (Status, error) {
	name, err := serviceName(role)
	if err != nil {
		return Status{}, err
	}

	output, err := scctl.RunOutput(ctx, "query", name, nil)
	if err != nil {
		return Status{}, err
	}

	state, active := scctl.ParseServiceState(output)
	return Status{
		Active: active,
		State:  serviceStateLabel(state),
		Detail: strings.TrimSpace(output),
	}, nil
}

func serviceName(role Role) (string, error) {
	switch role {
	case RoleClient:
		return "xp2p-client", nil
	case RoleServer:
		return "xp2p-server", nil
	default:
		return "", fmt.Errorf("unsupported role %q", role)
	}
}


func serviceStateLabel(state svc.State) string {
	switch state {
	case svc.Running:
		return "RUNNING"
	case svc.Stopped:
		return "STOPPED"
	case svc.StartPending:
		return "START_PENDING"
	case svc.StopPending:
		return "STOP_PENDING"
	case svc.ContinuePending:
		return "CONTINUE_PENDING"
	case svc.PausePending:
		return "PAUSE_PENDING"
	case svc.Paused:
		return "PAUSED"
	default:
		return "UNKNOWN"
	}
}
