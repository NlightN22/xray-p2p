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

type EndpointSetEnabledOptions struct {
	Target  string
	All     bool
	Enabled bool
}

func SetEndpointEnabled(ctx context.Context, opts EndpointSetEnabledOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !opts.All && strings.TrimSpace(opts.Target) == "" {
		return errors.New("endpoint hostname or tag is required")
	}

	configFile := config.ConfigPath(layout.ClientConfigFileName)
	state, err := loadClientInstallState(configFile)
	if err != nil {
		return err
	}
	changed, err := state.setEndpointsEnabled(opts.Target, opts.All, opts.Enabled)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return commitClientRuntimeState(ctx, state)
}

func (s *clientInstallState) setEndpointsEnabled(target string, all bool, enabled bool) (bool, error) {
	if len(s.Endpoints) == 0 {
		return false, ErrClientEndpointsMissing
	}
	changed := false
	trimmed := strings.TrimSpace(target)
	for idx := range s.Endpoints {
		if !all && !endpointMatchesTarget(s.Endpoints[idx], trimmed) {
			continue
		}
		disabled := !enabled
		if s.Endpoints[idx].Disabled == disabled {
			continue
		}
		s.Endpoints[idx].Disabled = disabled
		changed = true
	}
	if !all && !changed {
		for _, ep := range s.Endpoints {
			if endpointMatchesTarget(ep, trimmed) {
				return false, nil
			}
		}
		return false, fmt.Errorf("client endpoint %q not found", trimmed)
	}
	return changed, nil
}

func endpointMatchesTarget(ep clientEndpointRecord, target string) bool {
	return strings.EqualFold(ep.Hostname, target) || strings.EqualFold(ep.Tag, target)
}
