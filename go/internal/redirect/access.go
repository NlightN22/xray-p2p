package redirect

import (
	"fmt"
	"sort"
	"strings"
)

// AccessPolicy controls which authenticated users may match a redirect.
// Access is persisted as the fields specified by the server Desired format.
type AccessPolicy struct {
	Access string   `json:"access,omitempty" toml:"access,omitempty"`
	Users  []string `json:"users,omitempty" toml:"users,omitempty"`
	Groups []string `json:"groups,omitempty" toml:"groups,omitempty"`
}

func (p AccessPolicy) Normalized() (AccessPolicy, error) {
	p.Access = strings.ToLower(strings.TrimSpace(p.Access))
	p.Users = unique(p.Users)
	p.Groups = unique(p.Groups)
	if p.Access == "" {
		if len(p.Users)+len(p.Groups) > 0 {
			p.Access = "restricted"
		} else {
			p.Access = "all"
		}
	}
	if p.Access != "all" && p.Access != "restricted" {
		return AccessPolicy{}, fmt.Errorf("invalid redirect access %q", p.Access)
	}
	if p.Access == "restricted" && len(p.Users)+len(p.Groups) == 0 {
		return AccessPolicy{}, fmt.Errorf("restricted redirect access requires an allowed user or group")
	}
	return p, nil
}

func (p AccessPolicy) IsRestricted() bool {
	n, err := p.Normalized()
	return err == nil && n.Access == "restricted"
}
func (p AccessPolicy) EffectiveUsers() ([]string, error) {
	n, err := p.Normalized()
	if err != nil || n.Access != "restricted" {
		return nil, err
	}
	return n.Users, nil
}

func unique(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}
