//go:build windows

package windows

import (
	"context"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/ports"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type ServiceManager struct{}

func NewServiceManager() *ServiceManager {
	return &ServiceManager{}
}

func (m *ServiceManager) Start(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return withService(name, func(service *mgr.Service) error {
		return service.Start()
	})
}

func (m *ServiceManager) Stop(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return withService(name, func(service *mgr.Service) error {
		_, err := service.Control(svc.Stop)
		return err
	})
}

func (m *ServiceManager) Status(ctx context.Context, name string) (ports.ServiceInfo, error) {
	if err := ctx.Err(); err != nil {
		return ports.ServiceInfo{}, err
	}

	manager, err := mgr.Connect()
	if err != nil {
		return ports.ServiceInfo{}, err
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(name)
	if err != nil {
		return ports.ServiceInfo{}, err
	}
	defer service.Close()

	status, err := service.Query()
	if err != nil {
		return ports.ServiceInfo{}, err
	}

	displayName := ""
	if config, err := service.Config(); err == nil {
		displayName = config.DisplayName
	}

	return ports.ServiceInfo{
		Name:        name,
		DisplayName: displayName,
		State:       mapServiceState(status.State),
	}, nil
}

func (m *ServiceManager) List(ctx context.Context, names []string) ([]ports.ServiceInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}

	manager, err := mgr.Connect()
	if err != nil {
		return nil, err
	}
	defer manager.Disconnect()

	infos := make([]ports.ServiceInfo, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		service, err := manager.OpenService(name)
		if err != nil {
			return nil, fmt.Errorf("open service %s: %w", name, err)
		}

		status, err := service.Query()
		displayName := ""
		if config, cfgErr := service.Config(); cfgErr == nil {
			displayName = config.DisplayName
		}
		service.Close()
		if err != nil {
			return nil, fmt.Errorf("query service %s: %w", name, err)
		}

		infos = append(infos, ports.ServiceInfo{
			Name:        name,
			DisplayName: displayName,
			State:       mapServiceState(status.State),
		})
	}

	return infos, nil
}

func withService(name string, fn func(*mgr.Service) error) error {
	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(name)
	if err != nil {
		return err
	}
	defer service.Close()

	return fn(service)
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
