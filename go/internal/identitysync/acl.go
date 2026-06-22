package identitysync

import (
	"fmt"
	"sort"
	"strings"
)

const (
	MaxACLGroupsPerRedirect       = 100
	MaxACLGroupDepth              = 16
	MaxACLLabelsPerRedirect       = 10000
	MaxACLLabelsPerServer         = 100000
	MaxSerializedRoutingRuleBytes = 1024 * 1024
)

type ACLResolver struct {
	Generation *Generation
}

func (r ACLResolver) Resolve(explicitUsers, groupIDs []string) ([]string, error) {
	labels := map[string]string{}
	add := func(label string) {
		label = strings.TrimSpace(label)
		if label == "" {
			return
		}
		key := strings.ToLower(label)
		if _, ok := labels[key]; !ok {
			labels[key] = label
		}
	}
	for _, label := range explicitUsers {
		add(label)
	}
	if len(groupIDs) > MaxACLGroupsPerRedirect {
		return nil, fmt.Errorf("redirect access selects %d groups, limit is %d", len(groupIDs), MaxACLGroupsPerRedirect)
	}
	if r.Generation != nil && !r.Generation.Detached {
		visited := map[string]bool{}
		for _, groupID := range groupIDs {
			if err := r.collectGroupLabels(strings.TrimSpace(groupID), 1, visited, add); err != nil {
				return nil, err
			}
		}
	}
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		out = append(out, label)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	if len(out) > MaxACLLabelsPerRedirect {
		return nil, fmt.Errorf("redirect access resolves %d labels, limit is %d", len(out), MaxACLLabelsPerRedirect)
	}
	return out, nil
}

func (r ACLResolver) collectGroupLabels(groupID string, depth int, visited map[string]bool, add func(string)) error {
	if groupID == "" || r.Generation == nil {
		return nil
	}
	if depth > MaxACLGroupDepth {
		return fmt.Errorf("identity group nesting exceeds depth %d", MaxACLGroupDepth)
	}
	if visited[groupID] {
		return nil
	}
	visited[groupID] = true
	group, ok := r.Generation.Groups[groupID]
	if !ok {
		return nil
	}
	for _, subjectID := range group.DirectMembers {
		subject, ok := r.Generation.Subjects[subjectID]
		if !ok || !subject.Active || !subject.Provisioned {
			continue
		}
		add(subject.UserLabel)
	}
	for _, childID := range group.DirectGroups {
		if err := r.collectGroupLabels(strings.TrimSpace(childID), depth+1, visited, add); err != nil {
			return err
		}
	}
	return nil
}
