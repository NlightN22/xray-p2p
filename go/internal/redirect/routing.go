package redirect

import (
	"net"
	"sort"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/xrayrule"
)

// SortedRules returns active redirects in deterministic routing priority order.
func SortedRules(rules []Rule) []Rule {
	indexed := make([]indexedRule, 0, len(rules))
	for idx, rule := range rules {
		indexed = append(indexed, indexedRule{rule: rule, index: idx})
	}
	sort.SliceStable(indexed, func(i, j int) bool {
		left := indexed[i]
		right := indexed[j]
		if left.rule.Kind() != right.rule.Kind() {
			return left.rule.Kind() == KindDomain
		}
		if left.rule.Kind() == KindCIDR {
			leftPrefix := cidrPrefixLength(left.rule.Value())
			rightPrefix := cidrPrefixLength(right.rule.Value())
			if leftPrefix != rightPrefix {
				return leftPrefix > rightPrefix
			}
		}
		if leftValue, rightValue := left.rule.Value(), right.rule.Value(); leftValue != rightValue {
			return leftValue < rightValue
		}
		leftTag := strings.ToLower(strings.TrimSpace(left.rule.OutboundTag))
		rightTag := strings.ToLower(strings.TrimSpace(right.rule.OutboundTag))
		if leftTag != rightTag {
			return leftTag < rightTag
		}
		return left.index < right.index
	})

	out := make([]Rule, 0, len(indexed))
	for _, item := range indexed {
		out = append(out, item.rule)
	}
	return out
}

func BuildXrayRules(role string, rules []Rule) []any {
	sorted := SortedRules(rules)
	out := make([]any, 0, len(sorted))
	for _, rule := range sorted {
		entry := map[string]any{
			"type":        "field",
			"ruleTag":     xrayrule.Redirect(role, rule.OutboundTag, rule.Kind().String(), rule.Value()),
			"outboundTag": rule.OutboundTag,
		}
		switch rule.Kind() {
		case KindDomain:
			entry["domains"] = []string{domainPattern(rule.Value())}
		default:
			entry["ip"] = []string{rule.Value()}
		}
		out = append(out, entry)
	}
	return out
}

func domainPattern(value string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "full:") {
		return value
	}
	return "domain:" + value
}

type indexedRule struct {
	rule  Rule
	index int
}

func cidrPrefixLength(value string) int {
	_, network, err := net.ParseCIDR(strings.TrimSpace(value))
	if err != nil {
		return -1
	}
	ones, _ := network.Mask.Size()
	return ones
}
