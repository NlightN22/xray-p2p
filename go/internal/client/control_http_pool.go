package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

type controlHTTPPool struct {
	timeout  time.Duration
	socks    string
	clients  map[string]ownedhttp.OwnedClient
	testLeak bool
	sequence atomic.Uint64
	count    atomic.Int64
}

func newControlHTTPPool(timeout time.Duration, socksAddress string) *controlHTTPPool {
	pool := &controlHTTPPool{
		timeout: timeout,
		socks:   strings.TrimSpace(socksAddress),
		clients: make(map[string]ownedhttp.OwnedClient),
	}
	pool.testLeak = os.Getenv("XP2P_TEST_MODE") == "1" &&
		os.Getenv("XP2P_TEST_CONTROL_TRANSPORT_LEAK") == "1"
	return pool
}

func (p *controlHTTPPool) client(endpoint clientEndpointRecord) ownedhttp.OwnedClient {
	key := controlHTTPClientKey(endpoint, p.socks, p.timeout)
	if !p.testLeak {
		if client := p.clients[key]; client != nil {
			return client
		}
	} else {
		key = fmt.Sprintf("%s|test-leak-%d", key, p.sequence.Add(1))
	}
	var client ownedhttp.OwnedClient
	if p.socks == "" {
		client = controlHTTPClient(endpoint, p.timeout)
	} else {
		client = controlHTTPClientThroughSocks(endpoint, p.timeout, p.socks)
	}
	p.clients[key] = client
	p.count.Add(1)
	return client
}

func (p *controlHTTPPool) prune(ctx context.Context, endpoints []clientEndpointRecord) error {
	if p.testLeak {
		return nil
	}
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
		p.count.Add(-1)
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
		p.count.Add(-1)
	}
	return errors.Join(errs...)
}

func (p *controlHTTPPool) size() int64 {
	return p.count.Load()
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
