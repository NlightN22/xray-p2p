//go:build linux || windows

package client

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
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

	paths, err := resolvePendingClientPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	state, err := loadClientInstallState(paths.configFile)
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

	if err := state.save(paths.configFile); err != nil {
		return err
	}
	xrayCfg, err := ensureClientXrayConfig(paths.configFile)
	if err != nil {
		return err
	}
	endpointIPs, err := resolveEndpointIPMapWithCache(ctx, state.Endpoints)
	if err != nil {
		return err
	}
	if err := writeOutboundsConfig(filepath.Join(paths.configDir, "outbounds.json"), xrayCfg.DirectOutbound, state.Endpoints, endpointIPs, true); err != nil {
		return err
	}
	fullEnabled, fullTag, err := loadFullTunnelRouteSettings(paths.configFile)
	if err != nil {
		return err
	}
	routeEndpointIPs := map[string]fullTunnelEndpointIPs(nil)
	if fullEnabled {
		routeEndpointIPs, err = loadFullTunnelEndpointCache()
		if err != nil {
			return err
		}
	}
	if err := updateRoutingConfig(filepath.Join(paths.configDir, "routing.json"), xrayCfg.Routing, state.Endpoints, state.Redirects, state.Reverse, fullEnabled, fullTag, routeEndpointIPs, false); err != nil {
		return err
	}
	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
}
