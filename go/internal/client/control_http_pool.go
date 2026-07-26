package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

type controlHTTPPool struct {
	timeout time.Duration
	socks   string
	clients map[string]ownedhttp.OwnedClient
}

func newControlHTTPPool(timeout time.Duration, socksAddress string) *controlHTTPPool {
	return &controlHTTPPool{
		timeout: timeout,
		socks:   strings.TrimSpace(socksAddress),
		clients: make(map[string]ownedhttp.OwnedClient),
	}
}

func (p *controlHTTPPool) client(endpoint clientEndpointRecord) ownedhttp.OwnedClient {
	key := controlHTTPClientKey(endpoint, p.socks, p.timeout)
	if client := p.clients[key]; client != nil {
		return client
	}
	var client ownedhttp.OwnedClient
	if p.socks == "" {
		client = controlHTTPClient(endpoint, p.timeout)
	} else {
		client = controlHTTPClientThroughSocks(endpoint, p.timeout, p.socks)
	}
	p.clients[key] = client
	return client
}

func (p *controlHTTPPool) prune(ctx context.Context, endpoints []clientEndpointRecord) error {
	active := make(map[string]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		active[controlHTTPClientKey(endpoint, p.socks, p.timeout)] = struct{}{}
	}
	var errs []error
	for key, client := range p.clients {
		if _, ok := active[key]; ok {
			continue
		}
		if err := client.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown control HTTP client %q: %w", key, err))
		}
		delete(p.clients, key)
	}
	return errors.Join(errs...)
}

func (p *controlHTTPPool) shutdown(ctx context.Context) error {
	var errs []error
	for key, client := range p.clients {
		if err := client.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown control HTTP client %q: %w", key, err))
		}
		delete(p.clients, key)
	}
	return errors.Join(errs...)
}

func controlHTTPClientKey(endpoint clientEndpointRecord, socksAddress string, timeout time.Duration) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(endpoint.Hostname)),
		strings.ToLower(strings.TrimSpace(endpoint.ServerName)),
		strings.ToLower(strings.TrimSpace(endpoint.PinnedPeerCertSHA256)),
		fmt.Sprintf("%t", endpoint.AllowInsecure),
		strings.TrimSpace(socksAddress),
		timeout.String(),
	}
	return strings.Join(parts, "|")
}
