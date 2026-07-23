package client

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const DefaultDiagnosticsPort = "62022"

type heartbeatRunner struct {
	store     *heartbeat.Store
	configDir string
	endpoints []clientEndpointRecord
	interval  time.Duration
	timeout   time.Duration
	port      int
	socks     string
	backoff   map[string]heartbeatBackoff
	clients   map[string]*http.Client
}

func startHeartbeatLoop(ctx context.Context, installDir, configDir string, opts HeartbeatOptions) func() {
	if !opts.Enabled {
		return func() {}
	}

	runner, err := newHeartbeatRunner(installDir, configDir, opts)
	if err != nil {
		logging.Warn("client heartbeat disabled", "err", err)
		return func() {}
	}

	hbCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer runner.closeIdleHeartbeatClients()
		runner.loop(hbCtx)
	}()

	return func() {
		cancel()
		wg.Wait()
	}
}

func runHeartbeatOnce(ctx context.Context, installDir, configDir string, opts HeartbeatOptions) {
	if !opts.Enabled {
		return
	}
	runner, err := newHeartbeatRunner(installDir, configDir, opts)
	if err != nil {
		logging.Debug("client heartbeat once skipped", "err", err)
		return
	}
	defer runner.closeIdleHeartbeatClients()
	runner.runOnce(ctx)
}

func newHeartbeatRunner(installDir, configDir string, opts HeartbeatOptions) (*heartbeatRunner, error) {
	interval := opts.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	portStr := strings.TrimSpace(opts.Port)
	if portStr == "" {
		portStr = DefaultDiagnosticsPort
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid heartbeat port %q", portStr)
	}

	meta, err := loadLiveRuntimeMeta(configDir)
	if err != nil {
		return nil, err
	}
	state := runtimeDesiredToClientInstallState(meta.Desired)
	if len(state.Endpoints) == 0 {
		return nil, fmt.Errorf("no client endpoints configured")
	}
	socks := strings.TrimSpace(opts.SocksAddress)
	if socks == "" {
		return nil, fmt.Errorf("SOCKS tunnel is required for client heartbeat")
	}

	storeRoot := installDir
	if runtime.GOOS == "windows" {
		storeRoot = config.ConfigRoot()
	}
	storePath := filepath.Join(storeRoot, layout.ClientHeartbeatStateFileName)
	store, err := heartbeat.NewStore(storePath)
	if err != nil {
		return nil, err
	}
	endpoints := append([]clientEndpointRecord(nil), state.Endpoints...)

	return &heartbeatRunner{
		store:     store,
		configDir: configDir,
		endpoints: endpoints,
		interval:  interval,
		timeout:   timeout,
		port:      port,
		socks:     socks,
		backoff:   map[string]heartbeatBackoff{},
		clients:   map[string]*http.Client{},
	}, nil
}

func (r *heartbeatRunner) loop(ctx context.Context) {
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

func (r *heartbeatRunner) runOnce(ctx context.Context) {
	if meta, err := loadLiveRuntimeMeta(r.configDir); err == nil {
		state := runtimeDesiredToClientInstallState(meta.Desired)
		r.endpoints = append(r.endpoints[:0], state.Endpoints...)
		r.pruneHeartbeatControlClients(r.endpoints)
	} else {
		logging.Debug("client heartbeat metadata refresh failed", "err", err)
	}
	for idx, endpoint := range r.endpoints {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if endpoint.HeartbeatMode == heartbeat.ModeDisabled {
			continue
		}
		if r.endpointInBackoff(endpoint, time.Now()) {
			continue
		}
		r.pingEndpoint(ctx, endpoint, idx)
	}
}

func (r *heartbeatRunner) pingEndpoint(parent context.Context, endpoint clientEndpointRecord, index int) {
	ctx, cancel := context.WithTimeout(parent, r.timeout)
	defer cancel()

	targetHost, err := markerIPForIndex(index)
	if err != nil {
		logging.Warn("client heartbeat marker allocation failed", "host", endpoint.Hostname, "tag", endpoint.Tag, "err", err)
		_, _ = r.updateLocalHeartbeatAttempt(endpoint, heartbeatEndpointID(endpoint, "", DiagnosticsMarkerPort), boolPointer(false), 0, false, heartbeat.FailureStageMarker, err.Error())
		return
	}
	port := DiagnosticsMarkerPort
	endpointID := heartbeatEndpointID(endpoint, targetHost, port)
	reporter := heartbeatPingReporter{}
	client := r.heartbeatControlClient(endpoint)
	if err := ping.Run(ctx, targetHost, ping.Options{
		Count:                1,
		Timeout:              r.timeout,
		Port:                 port,
		SocksProxy:           r.socks,
		User:                 endpoint.User,
		Credential:           endpoint.Password,
		ServerName:           endpoint.ServerName,
		AllowInsecure:        endpoint.AllowInsecure,
		PinnedPeerCertSHA256: endpoint.PinnedPeerCertSHA256,
		Reporter:             &reporter,
		Silent:               true,
		HTTPClient:           client,
	}); err != nil {
		logging.Debug("client heartbeat failed", "host", endpoint.Hostname, "tag", endpoint.Tag, "err", err)
		r.recordHeartbeatFailure(endpoint)
		entry, persisted := r.updateLocalHeartbeatAttempt(endpoint, endpointID, boolPointer(false), 0, false, heartbeat.FailureStageProbe, err.Error())
		r.recordHeartbeatHealth(endpoint, entry, persisted)
		return
	}
	rttMillis := reporter.rttMillis()
	payload := heartbeat.Payload{
		Tag:        endpoint.Tag,
		Host:       endpoint.Hostname,
		User:       strings.TrimSpace(endpoint.User),
		ClientIP:   detectLocalIP(endpoint.Hostname),
		Timestamp:  time.Now().UTC(),
		RTTMillis:  rttMillis,
		EndpointID: endpointID,
		RTTValid:   true,
	}
	if err := postHeartbeat(ctx, targetHost, port, endpoint, endpoint.Password, payload, r.socks, client); err != nil {
		logging.Debug("client heartbeat report failed", "host", endpoint.Hostname, "tag", endpoint.Tag, "err", err)
		r.recordHeartbeatFailure(endpoint)
		entry, persisted := r.updateLocalHeartbeatAttempt(endpoint, endpointID, boolPointer(false), rttMillis, true, heartbeat.FailureStageReport, err.Error())
		r.recordHeartbeatHealth(endpoint, entry, persisted)
		return
	}
	r.recordHeartbeatSuccess(endpoint)
	entry, persisted := r.updateLocalHeartbeatAttempt(endpoint, endpointID, boolPointer(true), rttMillis, true, "", "")
	r.recordHeartbeatHealth(endpoint, entry, persisted)
}

func (r *heartbeatRunner) recordHeartbeatHealth(endpoint clientEndpointRecord, entry heartbeat.Entry, persisted bool) {
	if !persisted {
		return
	}
	switch entry.Status {
	case heartbeat.StatusHealthy:
		r.recordEndpointHealth(endpoint, true)
	case heartbeat.StatusUnhealthy:
		r.recordEndpointHealth(endpoint, false)
	}
}

func (r *heartbeatRunner) recordEndpointHealth(endpoint clientEndpointRecord, alive bool) {
	meta, err := loadLiveRuntimeMeta(r.configDir)
	if err != nil {
		return
	}
	if err := recordEndpointHealth(context.Background(), runtimeDesiredToClientInstallState(meta.Desired), endpoint.Tag, alive, time.Now().UTC()); err != nil {
		logging.Debug("endpoint group health update failed", "tag", endpoint.Tag, "err", err)
	}
}

func (r *heartbeatRunner) updateLocalHeartbeat(endpoint clientEndpointRecord, healthy bool, rttMillis int64) {
	_, _ = r.updateLocalHeartbeatAttempt(endpoint, "", boolPointer(healthy), rttMillis, healthy, "", "")
}

func (r *heartbeatRunner) updateLocalHeartbeatAttempt(endpoint clientEndpointRecord, endpointID string, healthy *bool, rttMillis int64, rttValid bool, stage heartbeat.FailureStage, failure string) (heartbeat.Entry, bool) {
	if r.store != nil {
		if healthy != nil && !*healthy && !rttValid {
			rttMillis = 0
		}
		payloadLocal := heartbeat.Payload{
			Tag:        endpoint.Tag,
			Host:       endpoint.Hostname,
			User:       strings.TrimSpace(endpoint.User),
			ClientIP:   detectLocalIP(endpoint.Hostname),
			Timestamp:  time.Now().UTC(),
			RTTMillis:  rttMillis,
			Healthy:    healthy,
			Mode:       endpoint.HeartbeatMode,
			Stage:      stage,
			Failure:    failure,
			EndpointID: endpointID,
			RTTValid:   rttValid,
		}
		entry, err := r.store.Update(payloadLocal)
		if err != nil {
			logging.Warn("client heartbeat persistence failed", "tag", endpoint.Tag, "failure_stage", heartbeat.FailureStagePersistence, "err", err)
			return entry, false
		}
		return entry, true
	}
	return heartbeat.Entry{}, false
}

func boolPointer(value bool) *bool {
	return &value
}

type heartbeatPingReporter struct {
	rtt time.Duration
}

func (r *heartbeatPingReporter) Report(_ context.Context, result ping.Result) error {
	r.rtt = result.RTT
	return nil
}

func (r heartbeatPingReporter) rttMillis() int64 {
	millis := r.rtt.Milliseconds()
	if millis <= 0 && r.rtt > 0 {
		return 1
	}
	return millis
}
