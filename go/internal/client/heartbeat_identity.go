package client

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

const heartbeatIdentityVersion = "v1"

func heartbeatEndpointID(endpoint clientEndpointRecord, markerHost string, markerPort int) string {
	values := []string{
		heartbeatIdentityVersion,
		strings.ToLower(strings.TrimSpace(endpoint.Tag)),
		strings.ToLower(strings.TrimSpace(endpoint.User)),
		strings.ToLower(strings.TrimSpace(endpoint.Hostname)),
		strings.ToLower(strings.TrimSpace(endpoint.Address)),
		strconv.Itoa(endpoint.Port),
		strings.ToLower(strings.TrimSpace(endpoint.Profile)),
		strings.ToLower(strings.TrimSpace(endpoint.Protocol)),
		strings.ToLower(strings.TrimSpace(endpoint.Transport)),
		strings.ToLower(strings.TrimSpace(endpoint.Security)),
		strings.ToLower(strings.TrimSpace(endpoint.Flow)),
		strings.ToLower(strings.TrimSpace(endpoint.ServerName)),
		strconv.FormatBool(endpoint.AllowInsecure),
		strings.ToLower(strings.TrimSpace(endpoint.PinnedPeerCertSHA256)),
		strings.ToLower(strings.TrimSpace(endpoint.VerifyPeerCertByName)),
		strings.ToLower(strings.Join(normalizeALPN(endpoint.ALPN), ",")),
		strings.ToLower(strings.TrimSpace(markerHost)),
		strconv.Itoa(markerPort),
	}
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return heartbeatIdentityVersion + ":" + hex.EncodeToString(sum[:])
}
