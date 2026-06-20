package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const defaultSubscriptionSyncInterval = 30 * time.Second

type subscriptionSyncRunner struct {
	configDir string
	statePath string
	interval  time.Duration
	timeout   time.Duration
}

func startSubscriptionSyncLoop(ctx context.Context, installDir, configDir string, opts HeartbeatOptions) func() {
	if !opts.Enabled {
		return func() {}
	}
	stateRoot := installDir
	if runtime.GOOS == "windows" {
		stateRoot = config.ConfigRoot()
	}
	runner := subscriptionSyncRunner{
		configDir: configDir,
		statePath: filepath.Join(stateRoot, layout.ClientHeartbeatStateFileName),
		interval:  defaultSubscriptionSyncInterval,
		timeout:   opts.Timeout,
	}
	if runner.timeout <= 0 {
		runner.timeout = 2 * time.Second
	}
	syncCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runner.loop(syncCtx)
	}()
	return func() {
		cancel()
		wg.Wait()
	}
}

func (r subscriptionSyncRunner) loop(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		r.runOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r subscriptionSyncRunner) runOnce(ctx context.Context) {
	meta, err := loadLiveRuntimeMeta(r.configDir)
	if err != nil {
		logging.Debug("subscription sync skipped", "err", err)
		return
	}
	state := runtimeDesiredToClientInstallState(meta.Desired)
	auth := controlAuthMap(meta.Control.AuthUsers)
	hb, err := heartbeat.Load(r.statePath)
	if err != nil {
		return
	}
	snapshots := hb.Snapshot(time.Now(), r.interval)
	dead := make(map[string]bool, len(snapshots))
	for _, snapshot := range snapshots {
		if !snapshot.Alive {
			dead[snapshot.Entry.Tag] = true
		}
	}
	for _, endpoint := range state.Endpoints {
		if !dead[endpoint.Tag] {
			continue
		}
		sub, err := fetchSubscription(ctx, endpoint, meta.Control.Endpoint.Port, auth[strings.TrimSpace(endpoint.User)], r.timeout)
		if err != nil {
			logging.Debug("subscription fetch failed", "tag", endpoint.Tag, "err", err)
			continue
		}
		if sub.Generation == "" || sub.Generation == meta.Control.Subscription.Generation {
			continue
		}
		candidate, err := subscriptionCandidate(state, endpoint, sub, auth[strings.TrimSpace(endpoint.User)])
		if err != nil {
			logging.Warn("subscription candidate rejected", "tag", endpoint.Tag, "err", err)
			continue
		}
		if _, err := commitClientRuntimeStateResult(ctx, candidate); err != nil {
			logging.Warn("subscription apply failed", "tag", endpoint.Tag, "generation", sub.Generation, "err", err)
			continue
		}
		logging.Info("subscription applied", "tag", endpoint.Tag, "generation", sub.Generation)
	}
}

func subscriptionCandidate(current clientInstallState, endpoint clientEndpointRecord, sub controlplane.Subscription, secret string) (clientInstallState, error) {
	if !strings.EqualFold(strings.TrimSpace(sub.Protocol), "trojan") {
		return clientInstallState{}, fmt.Errorf("unsupported subscription protocol %q", sub.Protocol)
	}
	if sub.Port <= 0 || sub.Port > 65535 {
		return clientInstallState{}, fmt.Errorf("invalid subscription port %d", sub.Port)
	}
	host := strings.TrimSpace(sub.Host)
	if host == "" {
		return clientInstallState{}, fmt.Errorf("subscription host is required")
	}
	if strings.TrimSpace(secret) == "" {
		return clientInstallState{}, fmt.Errorf("endpoint credential is missing from live control metadata")
	}
	updated := current
	updated.Endpoints = append([]clientEndpointRecord(nil), current.Endpoints...)
	for i := range updated.Endpoints {
		if updated.Endpoints[i].Tag != endpoint.Tag {
			continue
		}
		record := updated.Endpoints[i]
		record.Hostname = host
		record.Address = host
		record.Port = sub.Port
		record.Password = strings.TrimSpace(secret)
		if strings.TrimSpace(sub.ServerName) != "" {
			record.ServerName = strings.TrimSpace(sub.ServerName)
		}
		if strings.TrimSpace(sub.TLS.PinnedPeerCertSHA256) != "" {
			record.PinnedPeerCertSHA256 = strings.TrimSpace(sub.TLS.PinnedPeerCertSHA256)
			record.AllowInsecure = false
		} else {
			record.AllowInsecure = sub.TLS.ClientMayAllowInsecure
		}
		if strings.TrimSpace(sub.TLS.VerifyPeerCertByName) != "" {
			record.VerifyPeerCertByName = strings.TrimSpace(sub.TLS.VerifyPeerCertByName)
		}
		updated.Endpoints[i] = record
		updated.normalize()
		return updated, nil
	}
	return clientInstallState{}, fmt.Errorf("endpoint tag %q is not present in live runtime metadata", endpoint.Tag)
}

func fetchSubscription(ctx context.Context, endpoint clientEndpointRecord, port int, secret string, timeout time.Duration) (controlplane.Subscription, error) {
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
	resp, err := controlHTTPClient(endpoint, timeout).Do(req)
	if err != nil {
		return controlplane.Subscription{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return controlplane.Subscription{}, fmt.Errorf("subscription request failed: %s", resp.Status)
	}
	var sub controlplane.Subscription
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return controlplane.Subscription{}, err
	}
	return sub, nil
}
