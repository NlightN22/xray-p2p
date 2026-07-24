package runtimeboundary

import (
	"context"
	"testing"

	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func TestBoundaryContextTakesPrecedenceOverTestOverride(t *testing.T) {
	restore := SetForTesting(Boundary{
		ServiceStatus: func(context.Context, servicecontrol.Role) (servicecontrol.Status, error) {
			return servicecontrol.Status{State: "override"}, nil
		},
	})
	t.Cleanup(restore)
	ctx := WithBoundary(context.Background(), Boundary{
		ServiceStatus: func(context.Context, servicecontrol.Role) (servicecontrol.Status, error) {
			return servicecontrol.Status{State: "context"}, nil
		},
	})
	status, err := ServiceStatus(ctx, servicecontrol.RoleClient)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "context" {
		t.Fatalf("state=%q, want context", status.State)
	}
}

func TestBoundaryTestOverrideRestoresPreviousValue(t *testing.T) {
	restoreOuter := SetForTesting(Boundary{
		ServiceStatus: func(context.Context, servicecontrol.Role) (servicecontrol.Status, error) {
			return servicecontrol.Status{State: "outer"}, nil
		},
	})
	t.Cleanup(restoreOuter)
	restoreInner := SetForTesting(Boundary{
		ServiceStatus: func(context.Context, servicecontrol.Role) (servicecontrol.Status, error) {
			return servicecontrol.Status{State: "inner"}, nil
		},
	})
	restoreInner()
	status, err := ServiceStatus(context.Background(), servicecontrol.RoleServer)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "outer" {
		t.Fatalf("state=%q, want outer", status.State)
	}
}
