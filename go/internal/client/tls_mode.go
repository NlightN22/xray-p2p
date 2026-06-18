//go:build linux || windows

package client

import "strings"

const (
	TLSModeSystemCA   = "system-ca"
	TLSModePinned     = "pinned"
	TLSModePinnedName = "pinned+name"
	TLSModeInsecure   = "insecure"
	TLSModeUnknown    = "unknown"
)

func endpointTLSMode(endpoint clientEndpointRecord) string {
	if endpoint.AllowInsecure {
		return TLSModeInsecure
	}
	pinned := strings.TrimSpace(endpoint.PinnedPeerCertSHA256) != ""
	verifyName := strings.TrimSpace(endpoint.VerifyPeerCertByName) != ""
	switch {
	case pinned && verifyName:
		return TLSModePinnedName
	case pinned:
		return TLSModePinned
	case strings.TrimSpace(endpoint.ServerName) != "":
		return TLSModeSystemCA
	default:
		return TLSModeUnknown
	}
}
