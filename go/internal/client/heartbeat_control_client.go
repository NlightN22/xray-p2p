package client

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (r *heartbeatRunner) heartbeatControlClient(endpoint clientEndpointRecord) *http.Client {
	key := heartbeatControlClientKey(endpoint, r.socks, r.timeout)
	if r.clients == nil {
		r.clients = map[string]*http.Client{}
	}
	if client := r.clients[key]; client != nil {
		return client
	}
	client := controlHTTPClientThroughSocks(endpoint, r.timeout, r.socks)
	r.clients[key] = client
	return client
}

func (r *heartbeatRunner) pruneHeartbeatControlClients(endpoints []clientEndpointRecord) {
	if len(r.clients) == 0 {
		return
	}
	active := make(map[string]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		active[heartbeatControlClientKey(endpoint, r.socks, r.timeout)] = struct{}{}
	}
	for key, client := range r.clients {
		if _, ok := active[key]; ok {
			continue
		}
		client.CloseIdleConnections()
		delete(r.clients, key)
	}
}

func (r *heartbeatRunner) closeIdleHeartbeatClients() {
	for _, client := range r.clients {
		client.CloseIdleConnections()
	}
}

func heartbeatControlClientKey(endpoint clientEndpointRecord, socksAddress string, timeout time.Duration) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(endpoint.Tag)),
		strings.ToLower(strings.TrimSpace(endpoint.User)),
		strings.ToLower(strings.TrimSpace(endpoint.ServerName)),
		strings.ToLower(strings.TrimSpace(endpoint.PinnedPeerCertSHA256)),
		fmt.Sprintf("%t", endpoint.AllowInsecure),
		strings.TrimSpace(socksAddress),
		timeout.String(),
	}
	return strings.Join(parts, "|")
}
