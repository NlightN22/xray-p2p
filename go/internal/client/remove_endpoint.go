package client

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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
		return errors.New("xp2p: endpoint hostname or tag is required")
	}

	paths, err := resolveClientPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	state, err := loadClientInstallState(paths.stateFile)
	if err != nil {
		return err
	}
	if len(state.Endpoints) == 0 {
		return fmt.Errorf("xp2p: client endpoint %q not found", target)
	}

	record, removed := state.removeEndpoint(target)
	if !removed {
		return fmt.Errorf("xp2p: client endpoint %q not found", target)
	}

	state.removeRedirectsByTag(record.Tag)
	state.removeReverseChannelsByTag(record.Tag)

	if len(state.Endpoints) == 0 {
		return Remove(ctx, RemoveOptions{
			InstallDir: paths.installDir,
			ConfigDir:  opts.ConfigDir,
		})
	}

	if err := state.save(paths.stateFile); err != nil {
		return err
	}
	if err := writeOutboundsConfig(filepath.Join(paths.configDir, "outbounds.json"), state.Endpoints); err != nil {
		return err
	}
	return updateRoutingConfig(filepath.Join(paths.configDir, "routing.json"), state.Endpoints, state.Redirects, state.Reverse)
}
