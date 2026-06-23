package client

import (
	"fmt"
	"strings"
	"time"
)

const defaultEndpointGroupFailureThreshold = 2

type endpointSelectorState struct {
	Revision uint64                           `json:"revision"`
	Groups   map[string]endpointGroupSelector `json:"groups"`
}

type endpointGroupSelector struct {
	ActiveTag     string         `json:"active_tag,omitempty"`
	Failures      map[string]int `json:"failures,omitempty"`
	Successes     map[string]int `json:"successes,omitempty"`
	CooldownUntil time.Time      `json:"cooldown_until,omitempty"`
	ActiveSince   time.Time      `json:"active_since,omitempty"`
}

func (g endpointGroup) normalized() (endpointGroup, error) {
	g.GroupID = strings.TrimSpace(g.GroupID)
	g.Tag = strings.TrimSpace(g.Tag)
	if g.GroupID == "" || g.Tag == "" {
		return endpointGroup{}, fmt.Errorf("endpoint group ID and tag are required")
	}
	if g.Mode == "" {
		g.Mode = endpointGroupModeAutomatic
	}
	if g.Mode != endpointGroupModeAutomatic && g.Mode != endpointGroupModeManual && g.Mode != endpointGroupModeDisabled {
		return endpointGroup{}, fmt.Errorf("invalid endpoint group mode %q", g.Mode)
	}
	if g.FailureThreshold <= 0 {
		g.FailureThreshold = defaultEndpointGroupFailureThreshold
	}
	seen := make(map[string]struct{}, len(g.Members))
	members := make([]string, 0, len(g.Members))
	for _, member := range g.Members {
		member = strings.TrimSpace(member)
		key := strings.ToLower(member)
		if member == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			return endpointGroup{}, fmt.Errorf("endpoint group %q contains duplicate member %q", g.Tag, member)
		}
		seen[key] = struct{}{}
		members = append(members, member)
	}
	if len(members) == 0 {
		return endpointGroup{}, fmt.Errorf("endpoint group %q has no members", g.Tag)
	}
	g.Members = members
	return g, nil
}

func (s *clientInstallState) validateEndpointGroups() error {
	endpointTags := make(map[string]struct{}, len(s.Endpoints))
	for _, endpoint := range s.Endpoints {
		endpointTags[strings.ToLower(strings.TrimSpace(endpoint.Tag))] = struct{}{}
	}
	groupTags := make(map[string]struct{}, len(s.EndpointGroups))
	groupIDs := make(map[string]struct{}, len(s.EndpointGroups))
	for index := range s.EndpointGroups {
		group, err := s.EndpointGroups[index].normalized()
		if err != nil {
			return err
		}
		key := strings.ToLower(group.Tag)
		if _, exists := endpointTags[key]; exists {
			return fmt.Errorf("endpoint group tag %q conflicts with physical endpoint tag", group.Tag)
		}
		if _, exists := groupTags[key]; exists {
			return fmt.Errorf("duplicate endpoint group tag %q", group.Tag)
		}
		id := strings.ToLower(group.GroupID)
		if _, exists := groupIDs[id]; exists {
			return fmt.Errorf("duplicate endpoint group ID %q", group.GroupID)
		}
		for _, member := range group.Members {
			if _, exists := endpointTags[strings.ToLower(member)]; !exists {
				return fmt.Errorf("endpoint group %q references unknown member %q", group.Tag, member)
			}
		}
		groupTags[key], groupIDs[id] = struct{}{}, struct{}{}
		s.EndpointGroups[index] = group
	}
	return nil
}

func selectEndpointGroup(group endpointGroup, endpoints []clientEndpointRecord, state endpointGroupSelector, now time.Time) (string, bool) {
	group, err := group.normalized()
	if err != nil || group.Mode == endpointGroupModeDisabled {
		return "", false
	}
	available := make(map[string]bool, len(endpoints))
	for _, endpoint := range endpoints {
		available[strings.ToLower(strings.TrimSpace(endpoint.Tag))] = !endpoint.Disabled
	}
	memberAvailable := func(tag string) bool { return available[strings.ToLower(tag)] }
	if group.Mode == endpointGroupModeManual {
		if memberAvailable(group.ManualActiveTag) {
			return strings.TrimSpace(group.ManualActiveTag), true
		}
		return "", false
	}
	active := strings.TrimSpace(state.ActiveTag)
	if active != "" && memberAvailable(active) {
		if now.Before(state.CooldownUntil) || (group.MinimumHoldSeconds > 0 && now.Before(state.ActiveSince.Add(time.Duration(group.MinimumHoldSeconds)*time.Second))) || state.Failures[strings.ToLower(active)] < group.FailureThreshold {
			return active, true
		}
	}
	for _, tag := range group.Members {
		if !memberAvailable(tag) {
			continue
		}
		if state.Failures[strings.ToLower(tag)] < group.FailureThreshold {
			return tag, true
		}
	}
	return "", false
}

func endpointGroupTags(groups []endpointGroup) map[string]struct{} {
	tags := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if normalized, err := group.normalized(); err == nil {
			tags[strings.ToLower(normalized.Tag)] = struct{}{}
		}
	}
	return tags
}
