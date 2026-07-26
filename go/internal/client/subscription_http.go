package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

const subscriptionResponseLimit = 1 << 20

func fetchSubscription(ctx context.Context, client ownedhttp.Doer, endpoint clientEndpointRecord, port int, secret string) (controlplane.Subscription, error) {
	return fetchSubscriptionConditional(ctx, client, endpoint, port, secret, "")
}

func fetchSubscriptionConditional(ctx context.Context, client ownedhttp.Doer, endpoint clientEndpointRecord, port int, secret string, knownGeneration string) (controlplane.Subscription, error) {
	if client == nil {
		return controlplane.Subscription{}, fmt.Errorf("control HTTP client is required")
	}
	if port <= 0 {
		port = 62022
	}
	host := strings.TrimSpace(endpoint.Hostname)
	if host == "" {
		return controlplane.Subscription{}, fmt.Errorf("endpoint host is required")
	}
	url := "https://" + net.JoinHostPort(host, fmt.Sprintf("%d", port)) + controlplane.PathSubscription
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return controlplane.Subscription{}, err
	}
	if secret != "" {
		nonce, err := controlNonce()
		if err != nil {
			return controlplane.Subscription{}, err
		}
		if err := controlplane.ApplyHeaders(req, endpoint.User, secret, nonce, nil, time.Now().UTC()); err != nil {
			return controlplane.Subscription{}, err
		}
	}
	if strings.TrimSpace(knownGeneration) != "" {
		req.Header.Set(controlplane.HeaderKnownGeneration, strings.TrimSpace(knownGeneration))
	}
	resp, err := client.Do(req)
	if err != nil {
		return controlplane.Subscription{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return controlplane.Subscription{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, subscriptionResponseLimit))
		return controlplane.Subscription{}, fmt.Errorf("subscription request failed: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, subscriptionResponseLimit+1))
	if err != nil {
		return controlplane.Subscription{}, err
	}
	if len(body) > subscriptionResponseLimit {
		return controlplane.Subscription{}, fmt.Errorf("subscription response exceeds %d bytes", subscriptionResponseLimit)
	}
	var sub controlplane.Subscription
	if err := json.Unmarshal(body, &sub); err != nil {
		return controlplane.Subscription{}, err
	}
	return sub, nil
}
