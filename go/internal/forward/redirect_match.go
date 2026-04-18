package forward

import (
	"net/netip"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

// MatchesRedirect reports whether any redirect rule routes the supplied IP.
func MatchesRedirect(rules []redirect.Rule, addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	for _, rule := range rules {
		if rule.Kind() != redirect.KindCIDR {
			continue
		}
		prefix, err := netip.ParsePrefix(rule.CIDR)
		if err != nil {
			continue
		}
		if !prefix.Contains(addr) {
			continue
		}
		return true
	}
	return false
}
