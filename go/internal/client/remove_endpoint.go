//go:build linux || windows

package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

// RemoveEndpointOptions control removal of a specific endpoint.
type RemoveEndpointOptions struct {
	InstallDir string
	ConfigDir  string
	Target     string
}

// RemoveEndpoint deletes a single endpoint from the client state and updates configs.
func RemoveEndpoint(ctx context.Context, opts RemoveEndpointOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	target := strings.TrimSpace(opts.Target)
	if target == "" {
		return errors.New("endpoint hostname or tag is required")
	}

	configFile := config.ConfigPath(layout.ClientConfigFileName)
	state, err := loadClientInstallState(configFile)
	if err != nil {
		return err
	}
	if len(state.Endpoints) == 0 {
		return fmt.Errorf("client endpoint %q not found", target)
	}

	record, removed := state.removeEndpoint(target)
	if !removed {
		return fmt.Errorf("client endpoint %q not found", target)
	}

	state.removeRedirectsByTag(record.Tag)
	state.removeReverseChannelsByTag(record.Tag)

	return commitClientRuntimeState(ctx, state)
}
