package runtimeboundary

import (
	"context"
	"sync"

	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

// Boundary contains the external operations used by runtime-capable commands.
type Boundary struct {
	ApplyCandidate func(context.Context, xraylive.Options, xraylive.Artifacts) (xraylive.RuntimeApplyResult, error)
	ServiceStatus  func(context.Context, servicecontrol.Role) (servicecontrol.Status, error)
	RestartService func(context.Context, servicecontrol.Role) error
	NewRouting     xraylive.RoutingApplierFactory
	NewInbound     xraylive.InboundApplierFactory
	NewOutbound    xraylive.OutboundApplierFactory
	NewInboundUser xraylive.InboundUserApplierFactory
}

type boundaryContextKey struct{}

var (
	overrideMu sync.RWMutex
	override   *Boundary
)

// WithBoundary overrides runtime external operations for one command context.
func WithBoundary(ctx context.Context, boundary Boundary) context.Context {
	return context.WithValue(ctx, boundaryContextKey{}, boundary)
}

// SetForTesting overrides the external boundary until the returned restore function runs.
func SetForTesting(boundary Boundary) func() {
	overrideMu.Lock()
	previous := override
	override = &boundary
	overrideMu.Unlock()
	return func() {
		overrideMu.Lock()
		override = previous
		overrideMu.Unlock()
	}
}

// ApplyCandidate applies a candidate through the configured runtime boundary.
func ApplyCandidate(
	ctx context.Context,
	opts xraylive.Options,
	artifacts xraylive.Artifacts,
) (xraylive.RuntimeApplyResult, error) {
	if boundary, ok := fromContext(ctx); ok {
		if boundary.ApplyCandidate != nil {
			return boundary.ApplyCandidate(ctx, opts, artifacts)
		}
		if opts.NewApplier == nil {
			opts.NewApplier = boundary.NewRouting
		}
		if opts.NewInbound == nil {
			opts.NewInbound = boundary.NewInbound
		}
		if opts.NewOutbound == nil {
			opts.NewOutbound = boundary.NewOutbound
		}
		if opts.NewInboundUser == nil {
			opts.NewInboundUser = boundary.NewInboundUser
		}
	}
	return xraylive.ApplyCandidate(ctx, opts, artifacts)
}

// ServiceStatus reads service state through the configured external boundary.
func ServiceStatus(ctx context.Context, role servicecontrol.Role) (servicecontrol.Status, error) {
	if boundary, ok := fromContext(ctx); ok && boundary.ServiceStatus != nil {
		return boundary.ServiceStatus(ctx, role)
	}
	return servicecontrol.Default().Status(ctx, role)
}

// RestartService is exposed so command-level tests can prove restart fallback is absent.
func RestartService(ctx context.Context, role servicecontrol.Role) error {
	if boundary, ok := fromContext(ctx); ok && boundary.RestartService != nil {
		return boundary.RestartService(ctx, role)
	}
	return servicecontrol.ErrUnsupported
}

func fromContext(ctx context.Context) (Boundary, bool) {
	if ctx != nil {
		if boundary, ok := ctx.Value(boundaryContextKey{}).(Boundary); ok {
			return boundary, true
		}
	}
	overrideMu.RLock()
	current := override
	overrideMu.RUnlock()
	if current == nil {
		return Boundary{}, false
	}
	return *current, true
}
