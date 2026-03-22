package client

import "strings"

func endpointHost(endpoint clientEndpointRecord) string {
	host := strings.TrimSpace(endpoint.Hostname)
	if host != "" {
		return host
	}
	return strings.TrimSpace(endpoint.Address)
}
