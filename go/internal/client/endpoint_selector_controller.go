//go:build linux || windows

package client

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func recordEndpointHealth(ctx context.Context, desired clientInstallState, endpointTag string, alive bool, now time.Time) error {
	return apply.WithRoleLock(ctx, config.StateRoot(), apply.RoleClient, func() error {
		return recordEndpointHealthLocked(ctx, desired, endpointTag, alive, now)
	})
}

func recordEndpointHealthLocked(ctx context.Context, desired clientInstallState, endpointTag string, alive bool, now time.Time) error {
	if len(desired.EndpointGroups) == 0 {
		return nil
	}
	liveDir, err := config.LiveRoleDir("client")
	if err != nil {
		return err
	}
	path := filepath.Join(liveDir, layout.ClientEndpointSelectorStateFileName)
	state, err := loadEndpointSelectorState(path)
	if err != nil {
		return err
	}
	next := state
	next.Groups = make(map[string]endpointGroupSelector, len(state.Groups))
	for key, value := range state.Groups {
		next.Groups[key] = value
	}
	changed, switchNeeded := false, false
	for _, raw := range desired.EndpointGroups {
		group, err := raw.normalized()
		if err != nil {
			return err
		}
		member := false
		for _, tag := range group.Members {
			if strings.EqualFold(tag, endpointTag) {
				member = true
				break
			}
		}
		if !member {
			continue
		}
		key := strings.ToLower(group.GroupID)
		current := next.Groups[key]
		if current.Failures == nil {
			current.Failures = map[string]int{}
		}
		if current.Successes == nil {
			current.Successes = map[string]int{}
		}
		tagKey := strings.ToLower(strings.TrimSpace(endpointTag))
		if alive {
			current.Successes[tagKey]++
			current.Failures[tagKey] = 0
		} else {
			current.Failures[tagKey]++
			current.Successes[tagKey] = 0
		}
		before, beforeOK := selectEndpointGroup(group, desired.Endpoints, state.Groups[key], now)
		after, afterOK := selectEndpointGroup(group, desired.Endpoints, current, now)
		activeChanged := selectorActiveMismatch(current.ActiveTag, after, afterOK)
		if !beforeOK || before != after || activeChanged {
			if afterOK {
				current.ActiveTag, current.ActiveSince = after, now
				if before != after {
					current.CooldownUntil = now.Add(time.Duration(group.CooldownSeconds) * time.Second)
				}
			} else {
				current.ActiveTag = ""
			}
		}
		if current.ActiveTag == "" && afterOK {
			current.ActiveTag, current.ActiveSince = after, now
		}
		next.Groups[key] = current
		changed = true
		switchNeeded = switchNeeded || before != after || beforeOK != afterOK || activeChanged
	}
	if !changed {
		return nil
	}
	if !switchNeeded {
		return commitEndpointSelectorState(path, next)
	}
	artifacts, err := compileClientRuntimeCandidateWithSelector(desired, &next)
	if err != nil {
		return err
	}
	result, err := applyClientRuntimeCandidate(ctx, artifacts, nil)
	if err != nil {
		return err
	}
	if result != xraylive.RuntimeApplyApplied && result != xraylive.RuntimeApplyNoop {
		return xraylive.ResultError(result)
	}
	return commitEndpointSelectorState(path, next)
}

func selectorActiveMismatch(storedActive, selected string, selectedOK bool) bool {
	if !selectedOK {
		return strings.TrimSpace(storedActive) != ""
	}
	return !strings.EqualFold(strings.TrimSpace(storedActive), strings.TrimSpace(selected))
}
