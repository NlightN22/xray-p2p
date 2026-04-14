package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

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

	liveStatePath := filepath.Clean(config.LiveConfigPath(layout.ClientConfigFileName))
	pendingStatePath := filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName))
	state, err := loadClientInstallStateWithFallback(pendingStatePath, liveStatePath)
	if err != nil {
		return nil, err
	}
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
	endpoints = fillEndpointUsersFromOutbounds(endpoints, configDir)

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

func fillEndpointUsersFromOutbounds(endpoints []clientEndpointRecord, configDir string) []clientEndpointRecord {
	needsUser := false
	for _, endpoint := range endpoints {
		if strings.TrimSpace(endpoint.User) == "" {
			needsUser = true
			break
		}
	}
	if !needsUser || strings.TrimSpace(configDir) == "" {
		return endpoints
	}
	users, err := loadOutboundUsers(configDir)
	if err != nil {
		logging.Warn("client heartbeat: unable to load outbound users", "err", err)
		return endpoints
	}
	updated := make([]clientEndpointRecord, len(endpoints))
	for i, endpoint := range endpoints {
		endpoint.User = strings.TrimSpace(endpoint.User)
		if endpoint.User == "" {
			if user := users[strings.ToLower(strings.TrimSpace(endpoint.Tag))]; user != "" {
				endpoint.User = user
			}
		}
		updated[i] = endpoint
	}
	return updated
}

func loadOutboundUsers(configDir string) (map[string]string, error) {
	path := filepath.Join(configDir, "outbounds.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc struct {
		Outbounds []struct {
			Tag      string `json:"tag"`
			Settings struct {
				Servers []struct {
					Email string `json:"email"`
				} `json:"servers"`
			} `json:"settings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	users := make(map[string]string, len(doc.Outbounds))
	for _, outbound := range doc.Outbounds {
		tag := strings.ToLower(strings.TrimSpace(outbound.Tag))
		if tag == "" {
			continue
		}
		for _, server := range outbound.Settings.Servers {
			user := strings.TrimSpace(server.Email)
			if user == "" {
				continue
			}
			users[tag] = user
			break
		}
	}
	return users, nil
}

func detectLocalIP(targetHost string) string {
	targetIP := net.ParseIP(strings.TrimSpace(targetHost))
	if targetIP != nil {
		targetIP = targetIP.To4()
	}

	candidates := make([]string, 0, 4)

	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				var ipnet *net.IPNet
				switch v := addr.(type) {
				case *net.IPNet:
					ipnet = v
				case *net.IPAddr:
					ipnet = &net.IPNet{IP: v.IP, Mask: net.CIDRMask(32, 32)}
				}
				if ipnet == nil || ipnet.IP == nil || ipnet.IP.IsLoopback() {
					continue
				}
				ip := ipnet.IP.To4()
				if ip == nil {
					continue
				}
				if targetIP != nil && ipnet.Contains(targetIP) {
					if runtime.GOOS == "windows" {
						return ip.String()
					}
					candidates = append(candidates, ip.String())
					continue
				}
				candidates = append(candidates, ip.String())
			}
		}
	}
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP != nil {
			if v4 := addr.IP.To4(); v4 != nil {
				candidates = append(candidates, v4.String())
			}
		}
	}

	if runtime.GOOS != "windows" {
		for _, ip := range candidates {
			if strings.HasPrefix(ip, "10.0.2.") {
				return ip
			}
		}
	}

	for _, ip := range candidates {
		if runtime.GOOS == "windows" && strings.HasPrefix(ip, "10.0.2.") {
			continue
		}
		if isPrivateIPv4(ip) {
			return ip
		}
	}
	for _, ip := range candidates {
		if runtime.GOOS == "windows" && strings.HasPrefix(ip, "10.0.2.") {
			continue
		}
		if strings.HasPrefix(ip, "169.254.") {
			continue
		}
		return ip
	}
	return "127.0.0.1"
}

func isPrivateIPv4(ip string) bool {
	if strings.HasPrefix(ip, "10.") {
		return true
	}
	if strings.HasPrefix(ip, "192.168.") {
		return true
	}
	if strings.HasPrefix(ip, "172.") {
		parts := strings.Split(ip, ".")
		if len(parts) >= 2 {
			if oct, err := strconv.Atoi(parts[1]); err == nil && oct >= 16 && oct <= 31 {
				return true
			}
		}
	}
	return false
}
