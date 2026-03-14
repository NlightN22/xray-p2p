//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/ports"
	"golang.org/x/sys/windows"
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
	return withServiceAccess(name, windows.SERVICE_START|windows.SERVICE_QUERY_STATUS, func(service *mgr.Service) error {
		return service.Start()
	})
}

func (m *ServiceManager) Stop(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return withServiceAccess(name, windows.SERVICE_STOP|windows.SERVICE_QUERY_STATUS, func(service *mgr.Service) error {
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
		return ports.ServiceInfo{}, fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()

	service, err := openServiceWithAccess(manager, name, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		if isServiceMissing(err) {
			return ports.ServiceInfo{}, fmt.Errorf("service %s not installed: %w", name, err)
		}
		return ports.ServiceInfo{}, fmt.Errorf("open service %s: %w", name, err)
	}
	defer service.Close()

	status, err := service.Query()
	if err != nil {
		return ports.ServiceInfo{}, fmt.Errorf("query service %s: %w", name, err)
	}

	displayName := queryServiceDisplayName(manager, name)

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
		service, err := openServiceWithAccess(manager, name, windows.SERVICE_QUERY_STATUS)
		if err != nil {
			if isServiceMissing(err) {
				continue
			}
			return nil, fmt.Errorf("open service %s: %w", name, err)
		}

		status, err := service.Query()
		displayName := queryServiceDisplayName(manager, name)
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

	service, err := openServiceWithAccess(manager, name, windows.SERVICE_ALL_ACCESS)
	if err != nil {
		return err
	}
	defer service.Close()

	return fn(service)
}

func withServiceAccess(name string, access uint32, fn func(*mgr.Service) error) error {
	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()

	service, err := openServiceWithAccess(manager, name, access)
	if err != nil {
		return err
	}
	defer service.Close()

	return fn(service)
}

func openServiceWithAccess(manager *mgr.Mgr, name string, access uint32) (*mgr.Service, error) {
	handle, err := windows.OpenService(manager.Handle, windows.StringToUTF16Ptr(name), access)
	if err != nil {
		return nil, err
	}
	return &mgr.Service{Name: name, Handle: handle}, nil
}

func queryServiceDisplayName(manager *mgr.Mgr, name string) string {
	service, err := openServiceWithAccess(manager, name, windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		return ""
	}
	defer service.Close()
	config, err := service.Config()
	if err != nil {
		return ""
	}
	return config.DisplayName
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

func isServiceMissing(err error) bool {
	return errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST)
}
