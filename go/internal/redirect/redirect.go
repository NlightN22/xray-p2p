package redirect

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// Kind describes the supported redirect rule types.
type Kind int

const (
	KindCIDR Kind = iota
	KindDomain
)

// Rule captures a redirect rule persisted in state files.
type Rule struct {
	CIDR        string `json:"cidr,omitempty"`
	Domain      string `json:"domain,omitempty"`
	OutboundTag string `json:"outbound_tag"`
}

// Target identifies the normalized redirect selector.
type Target struct {
	Kind  Kind
	Value string
}

// Binding ties an outbound tag to a friendly host identifier.
type Binding struct {
	Tag  string
	Host string
}

var (
	ErrBindingNotSpecified = errors.New("redirect: tag or host is required")
	ErrBindingHostNotFound = errors.New("redirect: host binding not found")
	ErrBindingTagNotFound  = errors.New("redirect: outbound tag not found")
	ErrBindingTagMismatch  = errors.New("redirect: tag does not match host binding")
)

// ResolveBinding selects the outbound tag and host combination matching the provided filters.
func ResolveBinding(tag, host string, bindings []Binding) (Binding, error) {
	trimmedTag := strings.TrimSpace(tag)
	trimmedHost := strings.TrimSpace(host)
	if trimmedTag == "" && trimmedHost == "" {
		return Binding{}, ErrBindingNotSpecified
	}

	var matched Binding
	var found bool
	if trimmedHost != "" {
		for _, binding := range bindings {
			if strings.EqualFold(binding.Host, trimmedHost) {
				matched = binding
				found = true
				break
			}
		}
		if !found {
			return Binding{}, ErrBindingHostNotFound
		}
		trimmedTag = matched.Tag
	} else {
		for _, binding := range bindings {
			if strings.EqualFold(binding.Tag, trimmedTag) {
				matched = binding
				found = true
				break
			}
		}
		if !found {
			return Binding{}, ErrBindingTagNotFound
		}
	}

	result := Binding{
		Tag:  matched.Tag,
		Host: matched.Host,
	}
	if strings.TrimSpace(tag) != "" && !strings.EqualFold(tag, matched.Tag) {
		return result, ErrBindingTagMismatch
	}

	return result, nil
}

// ResolveRule normalizes cidr/domain inputs into a redirect target.
func ResolveRule(cidr, domain string) (Target, error) {
	hasCIDR := strings.TrimSpace(cidr) != ""
	hasDomain := strings.TrimSpace(domain) != ""
	switch {
	case hasCIDR && hasDomain:
		return Target{}, errors.New("xp2p: specify only one of --cidr or --domain")
	case !hasCIDR && !hasDomain:
		return Target{}, errors.New("xp2p: --cidr or --domain is required")
	case hasCIDR:
		normalized, err := NormalizeCIDR(cidr)
		if err != nil {
			return Target{}, err
		}
		return Target{Kind: KindCIDR, Value: normalized}, nil
	default:
		normalized, err := NormalizeDomain(domain)
		if err != nil {
			return Target{}, err
		}
		return Target{Kind: KindDomain, Value: normalized}, nil
	}
}

// NormalizeCIDR trims input and verifies CIDR formatting.
func NormalizeCIDR(value string) (string, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return "", errors.New("xp2p: --cidr is required")
	}
	_, network, err := net.ParseCIDR(clean)
	if err != nil {
		return "", fmt.Errorf("xp2p: invalid CIDR %q: %w", value, err)
	}
	return network.String(), nil
}

// NormalizeDomain verifies the domain is non-empty and lowercase.
func NormalizeDomain(value string) (string, error) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return "", errors.New("xp2p: --domain is required")
	}
	if strings.ContainsAny(clean, " \t\r\n") {
		return "", fmt.Errorf("xp2p: invalid domain %q", value)
	}
	return strings.ToLower(clean), nil
}

// Describe renders a human-readable label for a redirect target.
func Describe(kind Kind, value string) string {
	switch kind {
	case KindDomain:
		return fmt.Sprintf("domain %s", value)
	default:
		return fmt.Sprintf("CIDR %s", value)
	}
}

func (t Target) Describe() string {
	return Describe(t.Kind, t.Value)
}

func (t Target) Matches(rule Rule) bool {
	switch t.Kind {
	case KindDomain:
		return t.Kind == rule.Kind() && strings.EqualFold(rule.Value(), t.Value)
	default:
		return t.Kind == rule.Kind() && strings.EqualFold(rule.Value(), t.Value)
	}
}

// Kind returns the rule type (CIDR or domain).
func (r Rule) Kind() Kind {
	if strings.TrimSpace(r.Domain) != "" {
		return KindDomain
	}
	return KindCIDR
}

// Value returns the normalized CIDR or domain value.
func (r Rule) Value() string {
	if r.Kind() == KindDomain {
		return strings.ToLower(strings.TrimSpace(r.Domain))
	}
	return strings.TrimSpace(r.CIDR)
}

// AddRule appends a rule while enforcing uniqueness by target and outbound tag.
func AddRule(rules []Rule, rule Rule) ([]Rule, error) {
	kind := rule.Kind()
	value := rule.Value()
	if value == "" {
		return rules, errors.New("xp2p: redirect value is required")
	}
	trimmedTag := strings.TrimSpace(rule.OutboundTag)
	if trimmedTag == "" {
		return rules, errors.New("xp2p: outbound tag is required")
	}
	for _, existing := range rules {
		if existing.Kind() != kind {
			continue
		}
		if !strings.EqualFold(existing.Value(), value) {
			continue
		}
		if strings.EqualFold(existing.OutboundTag, trimmedTag) {
			return rules, fmt.Errorf("xp2p: redirect %s via %s already exists", Describe(kind, value), trimmedTag)
		}
	}
	rule.OutboundTag = trimmedTag
	return append(rules, rule), nil
}

// RemoveRule drops all rules matching the provided target and optional tag filter.
func RemoveRule(rules []Rule, target Target, tagFilter string) ([]Rule, bool) {
	if len(rules) == 0 {
		return rules, false
	}
	filtered := make([]Rule, 0, len(rules))
	trimmedTag := strings.TrimSpace(tagFilter)
	removed := false
	for _, rule := range rules {
		matchValue := target.Matches(rule)
		matchTag := trimmedTag == "" || strings.EqualFold(rule.OutboundTag, trimmedTag)
		if matchValue && matchTag {
			removed = true
			continue
		}
		filtered = append(filtered, rule)
	}
	return filtered, removed
}
