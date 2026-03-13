package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/ports"
)

type fakeServiceManager struct {
	services map[string]ports.ServiceInfo
	started  []string
	stopped  []string
}

func (f *fakeServiceManager) Start(_ context.Context, name string) error {
	f.started = append(f.started, name)
	return nil
}

func (f *fakeServiceManager) Stop(_ context.Context, name string) error {
	f.stopped = append(f.stopped, name)
	return nil
}

func (f *fakeServiceManager) Status(_ context.Context, name string) (ports.ServiceInfo, error) {
	info, ok := f.services[name]
	if !ok {
		return ports.ServiceInfo{}, errors.New("missing")
	}
	return info, nil
}

func (f *fakeServiceManager) List(_ context.Context, names []string) ([]ports.ServiceInfo, error) {
	out := make([]ports.ServiceInfo, 0, len(names))
	for _, name := range names {
		if info, ok := f.services[name]; ok {
			out = append(out, info)
		}
	}
	return out, nil
}

func TestServiceControlList(t *testing.T) {
	manager := &fakeServiceManager{
		services: map[string]ports.ServiceInfo{
			"xp2p-server": {Name: "xp2p-server", State: ports.ServiceStateRunning},
		},
	}
	control := NewServiceControl(manager, []string{"xp2p-server", "xp2p-client"})
	infos, err := control.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 || infos[0].Name != "xp2p-server" {
		t.Fatalf("unexpected list result: %+v", infos)
	}
}

func TestServiceControlGuardsUnknown(t *testing.T) {
	manager := &fakeServiceManager{}
	control := NewServiceControl(manager, []string{"xp2p-server"})
	if err := control.Start(context.Background(), "unknown"); err == nil {
		t.Fatal("expected error for unknown service")
	}
}
