//go:build windows

package windows

import (
	"context"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/platform/windows/scctl"
	"github.com/NlightN22/xray-p2p/go/internal/ports"
	"golang.org/x/sys/windows/svc"
)

type ServiceManager struct{}

func NewServiceManager() *ServiceManager {
	return &ServiceManager{}
}

func (m *ServiceManager) Start(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return scctl.Run(ctx, "start", name, scctl.AllowServiceAlreadyRunning)
}

func (m *ServiceManager) Stop(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return scctl.Run(ctx, "stop", name, scctl.AllowServiceNotStarted)
}

func (m *ServiceManager) Status(ctx context.Context, name string) (ports.ServiceInfo, error) {
	if err := ctx.Err(); err != nil {
		return ports.ServiceInfo{}, err
	}

	output, err := scctl.RunOutput(ctx, "query", name, nil)
	if err != nil {
		if scctl.IsServiceMissing(err) {
			return ports.ServiceInfo{}, fmt.Errorf("service %s not installed: %w", name, err)
		}
		return ports.ServiceInfo{}, fmt.Errorf("query service %s: %w", name, err)
	}

	return ports.ServiceInfo{
		Name:        name,
		DisplayName: "",
		State:       mapServiceState(parseServiceState(output)),
	}, nil
}

func (m *ServiceManager) List(ctx context.Context, names []string) ([]ports.ServiceInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}

	infos := make([]ports.ServiceInfo, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		info, err := m.Status(ctx, name)
		if err != nil {
			if scctl.IsServiceMissing(err) {
				continue
			}
			return nil, err
		}
		infos = append(infos, info)
	}

	return infos, nil
}

func parseServiceState(output string) svc.State {
	state, _ := scctl.ParseServiceState(output)
	return state
}

func mapServiceState(state svc.State) ports.ServiceState {
	switch state {
	case svc.Stopped:
		return ports.ServiceStateStopped
	case svc.StartPending:
		return ports.ServiceStateStartPending
	case svc.StopPending:
		return ports.ServiceStateStopPending
	case svc.Running:
		return ports.ServiceStateRunning
	case svc.ContinuePending:
		return ports.ServiceStateContinuePending
	case svc.PausePending:
		return ports.ServiceStatePausePending
	case svc.Paused:
		return ports.ServiceStatePaused
	default:
		return ports.ServiceStateUnknown
	}
}
