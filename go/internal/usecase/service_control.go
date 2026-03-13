package usecase

import (
	"context"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/ports"
)

type ServiceControl struct {
	manager  ports.ServiceManager
	services map[string]struct{}
	order    []string
}

func NewServiceControl(manager ports.ServiceManager, services []string) *ServiceControl {
	unique := make(map[string]struct{}, len(services))
	order := make([]string, 0, len(services))
	for _, name := range services {
		if name == "" {
			continue
		}
		if _, exists := unique[name]; exists {
			continue
		}
		unique[name] = struct{}{}
		order = append(order, name)
	}
	return &ServiceControl{
		manager:  manager,
		services: unique,
		order:    order,
	}
}

func (s *ServiceControl) List(ctx context.Context) ([]ports.ServiceInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.manager.List(ctx, s.order)
}

func (s *ServiceControl) Start(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.isAllowed(name) {
		return fmt.Errorf("service %q is not managed", name)
	}
	return s.manager.Start(ctx, name)
}

func (s *ServiceControl) Stop(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.isAllowed(name) {
		return fmt.Errorf("service %q is not managed", name)
	}
	return s.manager.Stop(ctx, name)
}

func (s *ServiceControl) Status(ctx context.Context, name string) (ports.ServiceInfo, error) {
	if err := ctx.Err(); err != nil {
		return ports.ServiceInfo{}, err
	}
	if !s.isAllowed(name) {
		return ports.ServiceInfo{}, fmt.Errorf("service %q is not managed", name)
	}
	return s.manager.Status(ctx, name)
}

func (s *ServiceControl) isAllowed(name string) bool {
	_, ok := s.services[name]
	return ok
}
