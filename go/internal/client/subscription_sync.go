package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

const defaultSubscriptionSyncInterval = 30 * time.Second

type subscriptionSyncRunner struct {
	configDir string
	statePath string
	interval  time.Duration
	timeout   time.Duration
	socks     string
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
	}
}

func runSubscriptionSyncOnce(ctx context.Context, installDir, configDir string, opts HeartbeatOptions) {
	runner, ok := newSubscriptionSyncRunner(installDir, configDir, opts)
	if !ok {
		return
	}
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
	}
	if runner.timeout <= 0 {
		runner.timeout = 2 * time.Second
	}
	return runner, true
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
	for index, endpoint := range state.Endpoints {
		secret := auth[strings.TrimSpace(endpoint.User)]
		if strings.TrimSpace(secret) == "" {
			continue
		}
		credential := secret
		rotationPending := false
		controlPort := remoteControlPort()
		rotation, rotationErr := fetchRotation(ctx, endpoint, controlPort, secret, r.timeout)
		if rotationErr != nil {
			logging.Debug("credential rotation check failed", "tag", endpoint.Tag, "err", rotationErr)
		} else if rotation.RotationPending {
			credential = rotation.ActiveCredential
			rotationPending = true
		}
		sub, err := fetchSubscriptionConditional(ctx, endpoint, controlPort, credential, r.timeout, meta.Control.Subscription.Generation)
		if err != nil {
			logging.Debug("subscription fetch failed", "tag", endpoint.Tag, "err", err)
			continue
		}
		logging.Debug("subscription fetch completed", "tag", endpoint.Tag, "known_generation", meta.Control.Subscription.Generation, "fetched_generation", sub.Generation, "has_topology", sub.Topology != nil)
		if sub.Generation == "" || sub.Generation == meta.Control.Subscription.Generation {
			continue
		}
		currentMeta, currentErr := loadLiveRuntimeMeta(r.configDir)
		if currentErr == nil && sub.Generation == currentMeta.Control.Subscription.Generation {
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
		if _, err := commitClientSubscriptionState(ctx, candidate); err != nil {
			logging.Warn("subscription apply failed", "tag", endpoint.Tag, "generation", sub.Generation, "err", err)
			continue
		}
		if rotationPending {
			if err := r.verifyRotationTunnel(ctx, candidate.Endpoints[index], index, credential); err != nil {
				logging.Debug("credential rotation tunnel verification failed", "tag", endpoint.Tag, "err", err)
				continue
			}
			if err := acknowledgeRotation(ctx, endpoint, controlPort, credential, r.timeout); err != nil {
				logging.Debug("credential rotation acknowledgement deferred", "tag", endpoint.Tag, "err", err)
			}
		}
		logging.Info("subscription applied", "tag", endpoint.Tag, "generation", sub.Generation)
	}
}

func (r subscriptionSyncRunner) verifyRotationTunnel(parent context.Context, endpoint clientEndpointRecord, index int, credential string) error {
	if r.socks == "" {
		return fmt.Errorf("SOCKS tunnel is unavailable for rotation verification")
	}
	marker, err := markerIPForIndex(index)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, r.timeout)
	defer cancel()
	return ping.Run(ctx, marker, ping.Options{Count: 1, Timeout: r.timeout, Port: DiagnosticsMarkerPort, SocksProxy: r.socks, User: endpoint.User, Credential: credential, ServerName: endpoint.ServerName, AllowInsecure: endpoint.AllowInsecure, PinnedPeerCertSHA256: endpoint.PinnedPeerCertSHA256, Silent: true})
}

func remoteControlPort() int {
	port, err := strconv.Atoi(DefaultDiagnosticsPort)
	if err != nil || port <= 0 || port > 65535 {
		return 62022
	}
	return port
}

func subscriptionCandidate(current clientInstallState, endpoint clientEndpointRecord, sub controlplane.Subscription, secret string) (clientInstallState, error) {
	if strings.TrimSpace(sub.Profile) == "" {
		sub.Profile = "trojan-tls"
	}
	if strings.TrimSpace(sub.Transport) == "" {
		sub.Transport = "tcp"
	}
	if strings.TrimSpace(sub.Security) == "" {
		sub.Security = "tls"
	}
	if !strings.EqualFold(strings.TrimSpace(sub.Protocol), "trojan") && !strings.EqualFold(strings.TrimSpace(sub.Protocol), "vless") {
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
	profile, err := tunnel.DefaultProfile(tunnel.Profile(strings.TrimSpace(sub.Profile)))
	if err != nil {
		return clientInstallState{}, err
	}
	if profile.Protocol != strings.TrimSpace(sub.Protocol) || profile.Transport != strings.TrimSpace(sub.Transport) || profile.Security != strings.TrimSpace(sub.Security) {
		return clientInstallState{}, fmt.Errorf("subscription profile fields do not match %q", sub.Profile)
	}
	if profile.Profile == tunnel.ProfileVLESSTLSVision {
		if err := tunnel.ValidateVLESSCredential(secret); err != nil {
			return clientInstallState{}, err
		}
		if strings.TrimSpace(sub.Parameters["flow"]) != "xtls-rprx-vision" {
			return clientInstallState{}, fmt.Errorf("VLESS TLS Vision subscription requires flow xtls-rprx-vision")
		}
	}
	updated := current
	updated.Endpoints = append([]clientEndpointRecord(nil), current.Endpoints...)
	for i := range updated.Endpoints {
		if updated.Endpoints[i].Tag != endpoint.Tag {
			continue
		}
		record := updated.Endpoints[i]
		record.Profile = strings.TrimSpace(sub.Profile)
		record.Protocol = strings.TrimSpace(sub.Protocol)
		record.Transport = strings.TrimSpace(sub.Transport)
		record.Security = strings.TrimSpace(sub.Security)
		record.Flow = strings.TrimSpace(sub.Parameters["flow"])
		record.Hostname = host
		record.Address = subscriptionAddress(endpoint, host)
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

func subscriptionAddress(endpoint clientEndpointRecord, host string) string {
	if strings.EqualFold(strings.TrimSpace(endpoint.Hostname), strings.TrimSpace(host)) && strings.TrimSpace(endpoint.Address) != "" {
		return strings.TrimSpace(endpoint.Address)
	}
	return strings.TrimSpace(host)
}

func fetchSubscription(ctx context.Context, endpoint clientEndpointRecord, port int, secret string, timeout time.Duration) (controlplane.Subscription, error) {
	return fetchSubscriptionConditional(ctx, endpoint, port, secret, timeout, "")
}

func fetchSubscriptionConditional(ctx context.Context, endpoint clientEndpointRecord, port int, secret string, timeout time.Duration, knownGeneration string) (controlplane.Subscription, error) {
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
	resp, err := controlHTTPClient(endpoint, timeout).Do(req)
	if err != nil {
		return controlplane.Subscription{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return controlplane.Subscription{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return controlplane.Subscription{}, fmt.Errorf("subscription request failed: %s", resp.Status)
	}
	var sub controlplane.Subscription
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return controlplane.Subscription{}, err
	}
	return sub, nil
}
