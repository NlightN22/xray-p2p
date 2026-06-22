package identitysync

import (
	"fmt"
	"strings"
)

func (p ProviderRef) Validate() error {
	if strings.TrimSpace(p.InstanceID) == "" {
		return fmt.Errorf("identity provider instance_id is required")
	}
	switch p.Kind {
	case ProviderLDAP, ProviderSCIM:
	default:
		return fmt.Errorf("identity provider kind %q is unsupported", p.Kind)
	}
	if len(p.Scope) > 100 {
		return fmt.Errorf("identity provider scope exceeds 100 groups")
	}
	seen := map[string]struct{}{}
	for _, group := range p.Scope {
		group = strings.TrimSpace(group)
		if group == "" {
			return fmt.Errorf("identity provider scope contains an empty group")
		}
		if _, ok := seen[group]; ok {
			return fmt.Errorf("identity provider scope contains duplicate group %q", group)
		}
		seen[group] = struct{}{}
	}
	return nil
}
