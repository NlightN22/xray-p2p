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

type ReverseSetEnabledOptions struct {
	Target  string
	All     bool
	Enabled bool
}

func SetReverseEnabled(opts ReverseSetEnabledOptions) error {
	if !opts.All && strings.TrimSpace(opts.Target) == "" {
		return errors.New("reverse tag, user, or host is required")
	}
	configFile := config.ConfigPath(layout.ClientConfigFileName)
	state, err := loadClientInstallState(configFile)
	if err != nil {
		return err
	}
	changed, err := state.setReverseEnabled(opts.Target, opts.All, opts.Enabled)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return commitClientRuntimeState(context.Background(), state)
}

func (s *clientInstallState) setReverseEnabled(target string, all bool, enabled bool) (bool, error) {
	if len(s.Reverse) == 0 {
		return false, errors.New("no reverse tunnels configured")
	}
	changed := false
	found := false
	for key, channel := range s.Reverse {
		if !all && !clientReverseMatches(channel, target) {
			continue
		}
		found = true
		channel.Disabled = !enabled
		if s.Reverse[key].Disabled != channel.Disabled {
			s.Reverse[key] = channel
			changed = true
		}
	}
	if !all && !found {
		return false, fmt.Errorf("reverse tunnel %q not found", strings.TrimSpace(target))
	}
	return changed, nil
}

func clientReverseMatches(channel clientReverseChannel, target string) bool {
	trimmed := strings.TrimSpace(target)
	return strings.EqualFold(channel.Tag, trimmed) ||
		strings.EqualFold(channel.UserID, trimmed) ||
		strings.EqualFold(channel.Host, trimmed)
}
