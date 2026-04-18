package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
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
	endpoints []clientEndpointRecord
	interval  time.Duration
	timeout   time.Duration
	port      int
	socks     string
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
		runner.loop(hbCtx)
	}()

	return func() {
		cancel()
		wg.Wait()
	}
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
		endpoints: endpoints,
		interval:  interval,
		timeout:   timeout,
		port:      port,
		socks:     strings.TrimSpace(opts.SocksAddress),
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
	for idx, endpoint := range r.endpoints {
		select {
		case <-ctx.Done():
			return
		default:
		}
		r.pingEndpoint(ctx, endpoint, idx)
	}
}

func (r *heartbeatRunner) pingEndpoint(parent context.Context, endpoint clientEndpointRecord, index int) {
	ctx, cancel := context.WithTimeout(parent, r.timeout)
	defer cancel()

	reporter := newHeartbeatReporter(endpoint, r.store)
	targetHost := endpoint.Hostname
	port := r.port
	opts := ping.Options{
		Count:    1,
		Timeout:  r.timeout,
		Proto:    "tcp",
		Port:     port,
		Reporter: reporter,
		Silent:   true,
	}

	if r.socks != "" {
		markerIP, err := markerIPForIndex(index)
		if err != nil {
			logging.Warn("client heartbeat marker allocation failed", "host", endpoint.Hostname, "tag", endpoint.Tag, "err", err)
			return
		}
		targetHost = markerIP
		port = DiagnosticsMarkerPort
		opts.Port = port
		opts.SocksProxy = r.socks
		if err := ping.Run(ctx, targetHost, opts); err != nil {
			logging.Debug("client heartbeat failed", "host", endpoint.Hostname, "tag", endpoint.Tag, "err", err)
		}
		return
	}

	if err := ping.Run(ctx, targetHost, opts); err != nil {
		logging.Debug("client heartbeat failed", "host", endpoint.Hostname, "tag", endpoint.Tag, "err", err)
	}
}

type heartbeatReporter struct {
	endpoint clientEndpointRecord
	store    *heartbeat.Store
}

func newHeartbeatReporter(endpoint clientEndpointRecord, store *heartbeat.Store) heartbeatReporter {
	return heartbeatReporter{
		endpoint: endpoint,
		store:    store,
	}
}

func (r heartbeatReporter) Report(ctx context.Context, conn net.Conn, result ping.Result) error {
	payload := heartbeat.Payload{
		Tag:       r.endpoint.Tag,
		Host:      r.endpoint.Hostname,
		User:      strings.TrimSpace(r.endpoint.User),
		ClientIP:  detectLocalIP(r.endpoint.Hostname),
		Timestamp: time.Now().UTC(),
		RTTMillis: result.RTT.Milliseconds(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return err
	}
	if r.store != nil {
		payloadLocal := payload
		payloadLocal.Timestamp = time.Time{}
		if _, err := r.store.Update(payloadLocal); err != nil {
			logging.Warn("client heartbeat: failed to update local store", "tag", payload.Tag, "err", err)
		}
	}
	return nil
}
