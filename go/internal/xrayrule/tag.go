package xrayrule

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func Redirect(role, outboundTag, kind, value string, access ...string) string {
	return tag(append([]string{role, "redirect", outboundTag, kind, value}, access...)...)
}

func EndpointBypass(role, endpointTag, target string) string {
	return tag(role, "endpoint-bypass", endpointTag, target)
}

func DiagnosticsMarker(role, endpointTag string) string {
	return tag(role, "diagnostics-marker", endpointTag)
}

func ReverseDomain(role, reverseTag, endpointTag, domain string) string {
	return tag(role, "reverse-domain", reverseTag, endpointTag, domain)
}

func ReverseDirect(role, reverseTag string) string {
	return tag(role, "reverse-direct", reverseTag)
}

func ServerReverse(role, reverseTag, domain, user string) string {
	return tag(role, "reverse", reverseTag, domain, user)
}

func FullTunnel(role, outboundTag string) string {
	return tag(role, "full-tunnel", outboundTag)
}

func WindowsDirect(role, outboundTag, network string) string {
	return tag(role, "windows-direct", outboundTag, network)
}

func tag(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.ToLower(strings.TrimSpace(part))
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\x00")))
	return "xp2p-" + hex.EncodeToString(sum[:8])
}
