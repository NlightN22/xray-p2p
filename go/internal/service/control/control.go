package control

import (
	"context"
	"errors"
	"sync"
)

var (
	defaultOverrideMu sync.RWMutex
	defaultOverride   Controller
)

// Role identifies the managed service.
type Role string

const (
	// RoleClient targets the xp2p client service.
	RoleClient Role = "client"
	// RoleServer targets the xp2p server service.
	RoleServer Role = "server"
)

// ErrUnsupported indicates that the current platform has no service manager integration.
var ErrUnsupported = errors.New("service manager is not available on this platform")

// Status describes the current unit state.
type Status struct {
	Active bool
	State  string
	Detail string
}

// Controller abstracts service control operations.
type Controller interface {
	Start(ctx context.Context, role Role) error
	Stop(ctx context.Context, role Role) error
	Status(ctx context.Context, role Role) (Status, error)
}

// Default returns the platform-specific controller.
func Default() Controller {
	defaultOverrideMu.RLock()
	override := defaultOverride
	defaultOverrideMu.RUnlock()
	if override != nil {
		return override
	}
	return defaultController()
}

// SetDefaultForTesting overrides the platform controller until the returned restore function is called.
func SetDefaultForTesting(controller Controller) func() {
	defaultOverrideMu.Lock()
	previous := defaultOverride
	defaultOverride = controller
	defaultOverrideMu.Unlock()
	return func() {
		defaultOverrideMu.Lock()
		defaultOverride = previous
		defaultOverrideMu.Unlock()
	}
}
