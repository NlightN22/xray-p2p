package client

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
	subscriptiondomain "github.com/NlightN22/xray-p2p/go/internal/subscription"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

const defaultSubscriptionSyncInterval = 30 * time.Second

type subscriptionSyncRunner struct {
	configDir string
	statePath string
	interval  time.Duration
	timeout   time.Duration
	socks     string
	clients   *controlHTTPPool
	commit    func(context.Context, clientInstallState, func(context.Context) error) (xraylive.RuntimeApplyResult, error)
	ack       func(context.Context, ownedhttp.Doer, clientEndpointRecord, int, string) error
	probe     func(context.Context, clientEndpointRecord, int, string) error
}

func startSubscriptionSyncLoop(ctx context.Context, installDir, configDir string, opts HeartbeatOptions) func() {
	runner, ok := newSubscriptionSyncRunner(installDir, configDir, opts)
	if !ok {
		return func() {}
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
		runner.shutdown()
	}
}

func runSubscriptionSyncOnce(ctx context.Context, installDir, configDir string, opts HeartbeatOptions) {
	runner, ok := newSubscriptionSyncRunner(installDir, configDir, opts)
	if !ok {
		return
	}
	defer runner.shutdown()
	runner.runOnce(ctx)
}

func newSubscriptionSyncRunner(installDir, configDir string, opts HeartbeatOptions) (subscriptionSyncRunner, bool) {
	if !opts.Enabled {
		return subscriptionSyncRunner{}, false
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
		socks:     strings.TrimSpace(opts.SocksAddress),
		commit:    commitClientSubscriptionStateVerified,
		ack:       acknowledgeRotation,
	}
	if runner.timeout <= 0 {
		runner.timeout = 2 * time.Second
	}
	runner.clients = newControlHTTPPool(runner.timeout, "")
	return runner, true
}

func (r subscriptionSyncRunner) shutdown() {
	if r.clients == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	if err := r.clients.shutdown(ctx); err != nil {
		logging.Debug("subscription HTTP client shutdown failed", "err", err)
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
	pruneCtx, pruneCancel := context.WithTimeout(ctx, r.timeout)
	if err := r.clients.prune(pruneCtx, state.Endpoints); err != nil {
		logging.Debug("stale subscription client shutdown failed", "err", err)
	}
	pruneCancel()
	auth := controlAuthMap(meta.Control.AuthUsers)
	for index, endpoint := range state.Endpoints {
		secret := auth[strings.TrimSpace(endpoint.User)]
		if strings.TrimSpace(secret) == "" {
			continue
		}
		credential := secret
		rotationPending := false
		controlPort := remoteControlPort()
		client := r.clients.client(endpoint)
		rotation, rotationErr := fetchRotation(ctx, client, endpoint, controlPort, secret)
		if rotationErr != nil {
			logging.Debug("credential rotation check failed", "tag", endpoint.Tag, "err", rotationErr)
		} else if rotation.RotationPending {
			credential = rotation.ActiveCredential
			rotationPending = true
		}
		sub, err := fetchSubscriptionConditional(ctx, client, endpoint, controlPort, credential, meta.Control.Subscription.Generation)
		if err != nil {
			logging.Debug("subscription fetch failed", "tag", endpoint.Tag, "err", err)
			continue
		}
		logging.Debug("subscription fetch completed", "tag", endpoint.Tag, "known_generation", meta.Control.Subscription.Generation, "fetched_generation", sub.Generation, "has_topology", sub.Topology != nil)
		if rotationPending && (sub.Generation == "" || sub.Generation == meta.Control.Subscription.Generation) {
			sub = meta.Control.Subscription
		}
		if sub.Generation == "" || (sub.Generation == meta.Control.Subscription.Generation && !rotationPending) {
			continue
		}
		currentMeta, currentErr := loadLiveRuntimeMeta(r.configDir)
		if !rotationPending && currentErr == nil && sub.Generation == currentMeta.Control.Subscription.Generation {
			continue
		}
		candidate, err := subscriptionCandidate(state, endpoint, sub, credential)
		if err != nil {
			logging.Warn("subscription candidate rejected", "tag", endpoint.Tag, "err", err)
			continue
		}
		candidate, err = applySubscriptionTopology(candidate, endpoint, sub, credential)
		if err != nil {
			logging.Warn("subscription topology rejected", "tag", endpoint.Tag, "err", err)
			continue
		}
		result, ackErr, err := r.applySubscriptionCandidate(ctx, candidate, endpoint, index, controlPort, credential, rotationPending)
		if err != nil {
			logging.Warn("subscription apply failed", "tag", endpoint.Tag, "generation", sub.Generation, "err", err)
			continue
		}
		if rotationPending {
			if result != xraylive.RuntimeApplyApplied && result != xraylive.RuntimeApplyNoop {
				logging.Debug("credential rotation acknowledgement deferred", "tag", endpoint.Tag, "apply", result)
				continue
			}
			if ackErr != nil {
				logging.Debug("credential rotation acknowledgement deferred", "tag", endpoint.Tag, "err", ackErr)
			}
		}
		logging.Info("subscription applied", "tag", endpoint.Tag, "generation", sub.Generation)
	}
}

func remoteControlPort() int {
	port, err := strconv.Atoi(DefaultDiagnosticsPort)
	if err != nil || port <= 0 || port > 65535 {
		return 62022
	}
	return port
}

func subscriptionCandidate(current clientInstallState, endpoint clientEndpointRecord, sub controlplane.Subscription, secret string) (clientInstallState, error) {
	data, err := json.Marshal(sub)
	if err != nil {
		return clientInstallState{}, err
	}
	snapshot, err := (subscriptiondomain.XP2PControlDecoder{Credential: secret, UserLabel: endpoint.User}).Decode(subscriptiondomain.RawSnapshot{
		Source: subscriptiondomain.SourceRef{ID: endpoint.Tag, Adapter: subscriptiondomain.AdapterXP2PControl},
		Data:   data, FetchedAt: time.Now().UTC(),
	})
	if err != nil {
		return clientInstallState{}, err
	}
	offer := snapshot.Offers[0]
	profile := offer.Endpoint
	updated := current
	updated.Endpoints = append([]clientEndpointRecord(nil), current.Endpoints...)
	for i := range updated.Endpoints {
		if updated.Endpoints[i].Tag != endpoint.Tag {
			continue
		}
		record := updated.Endpoints[i]
		record.Profile = string(profile.Profile)
		record.Protocol = profile.Protocol
		record.Transport = profile.Transport
		record.Security = profile.Security
		record.Flow = profile.Metadata["flow"]
		record.Hostname = profile.Host
		record.Address = subscriptionAddress(endpoint, profile.Host)
		record.Port = profile.Port
		record.Password = offer.Credential
		record.ServerName = profile.ServerName
		record.ALPN = append([]string(nil), profile.TLS.ALPN...)
		record.PinnedPeerCertSHA256 = profile.TLS.PinnedPeerCertSHA256
		record.VerifyPeerCertByName = profile.TLS.VerifyPeerCertByName
		record.AllowInsecure = profile.TLS.AllowInsecure
		updated.Endpoints[i] = record
		updated.normalize()
		return updated, nil
	}
	return clientInstallState{}, fmt.Errorf("endpoint tag %q is not present in live runtime metadata", endpoint.Tag)
}

func subscriptionAddress(endpoint clientEndpointRecord, host string) string {
	if strings.EqualFold(strings.TrimSpace(endpoint.Hostname), strings.TrimSpace(host)) && strings.TrimSpace(endpoint.Address) != "" {
		return strings.TrimSpace(endpoint.Address)
	}
	return strings.TrimSpace(host)
}
