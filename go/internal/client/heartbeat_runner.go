package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
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

func startHeartbeatLoop(ctx context.Context, installDir string, opts HeartbeatOptions) func() {
	if !opts.Enabled {
		return func() {}
	}

	runner, err := newHeartbeatRunner(installDir, opts)
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

func newHeartbeatRunner(installDir string, opts HeartbeatOptions) (*heartbeatRunner, error) {
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

	statePath := filepath.Join(installDir, installstate.FileNameForKind(installstate.KindClient))
	state, err := loadClientInstallState(statePath)
	if err != nil {
		return nil, err
	}
	if len(state.Endpoints) == 0 {
		return nil, fmt.Errorf("no client endpoints configured")
	}

	storePath := filepath.Join(installDir, layout.HeartbeatStateFileName)
	store, err := heartbeat.NewStore(storePath)
	if err != nil {
		return nil, err
	}

	return &heartbeatRunner{
		store:     store,
		endpoints: append([]clientEndpointRecord(nil), state.Endpoints...),
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
	for _, endpoint := range r.endpoints {
		select {
		case <-ctx.Done():
			return
		default:
		}
		r.pingEndpoint(ctx, endpoint)
	}
}

func (r *heartbeatRunner) pingEndpoint(parent context.Context, endpoint clientEndpointRecord) {
	ctx, cancel := context.WithTimeout(parent, r.timeout)
	defer cancel()

	reporter := newHeartbeatReporter(endpoint, r.store)
	opts := ping.Options{
		Count:    1,
		Timeout:  r.timeout,
		Proto:    "tcp",
		Port:     r.port,
		Reporter: reporter,
		Silent:   true,
	}

	if err := ping.Run(ctx, endpoint.Hostname, opts); err != nil {
		if r.socks != "" {
			opts.SocksProxy = r.socks
			if err := ping.Run(ctx, endpoint.Hostname, opts); err != nil {
				logging.Debug("client heartbeat failed", "host", endpoint.Hostname, "tag", endpoint.Tag, "err", err)
			}
		} else {
			logging.Debug("client heartbeat failed", "host", endpoint.Hostname, "tag", endpoint.Tag, "err", err)
		}
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
		User:      r.endpoint.User,
		ClientIP:  detectLocalIP(),
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
		if _, err := r.store.Update(payload); err != nil {
			logging.Warn("client heartbeat: failed to update local store", "tag", payload.Tag, "err", err)
		}
	}
	return nil
}

func detectLocalIP() string {
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() {
					continue
				}
				if v4 := ip.To4(); v4 != nil {
					return v4.String()
				}
			}
		}
	}
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP != nil {
			return addr.IP.String()
		}
	}
	return "127.0.0.1"
}
