//go:build windows || linux

package server

import (
	"errors"
	"fmt"
	"strings"
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
	store, err := openReverseStore("")
	if err != nil {
		return err
	}
	changed, err := store.setReverseEnabled(opts.Target, opts.All, opts.Enabled)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if err := store.save(); err != nil {
		return err
	}
	return writeServerApplyRequest()
}

func (s *reverseStore) setReverseEnabled(target string, all bool, enabled bool) (bool, error) {
	if len(s.state) == 0 {
		return false, errors.New("no reverse tunnels configured")
	}
	changed := false
	found := false
	for key, channel := range s.state {
		if !all && !serverReverseMatches(channel, target) {
			continue
		}
		found = true
		channel.Disabled = !enabled
		if s.state[key].Disabled != channel.Disabled {
			s.state[key] = channel
			changed = true
		}
	}
	if !all && !found {
		return false, fmt.Errorf("reverse tunnel %q not found", strings.TrimSpace(target))
	}
	return changed, nil
}

func serverReverseMatches(channel serverReverseChannel, target string) bool {
	trimmed := strings.TrimSpace(target)
	return strings.EqualFold(channel.Tag, trimmed) ||
		strings.EqualFold(channel.UserID, trimmed) ||
		strings.EqualFold(channel.Host, trimmed)
}
