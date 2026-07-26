package client

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

func (r *heartbeatRunner) heartbeatControlClient(endpoint clientEndpointRecord) ownedhttp.OwnedClient {
	if r.clients == nil {
		r.clients = newControlHTTPPool(r.timeout, r.socks)
	}
	return r.clients.client(endpoint)
}

func (r *heartbeatRunner) pruneHeartbeatControlClients(endpoints []clientEndpointRecord) {
	if r.clients == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	if err := r.clients.prune(ctx, endpoints); err != nil {
		logging.Debug("stale heartbeat client shutdown failed", "err", err)
	}
}
