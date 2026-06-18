//go:build xray_smoke

package xrayapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	coreconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/coreconfig"
)

func TestBundledXrayAPI(t *testing.T) {
	xrayPath := bundledXrayPath(t)
	apiAddress := freeTCPAddress(t)
	workDir := t.TempDir()
	configPath := filepath.Join(workDir, "xray-api-smoke.json")
	writeSmokeXrayConfig(t, configPath, workDir, apiAddress)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, xrayPath, "run", "-config", configPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start xray: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	defer func() {
		cancel()
		<-done
	}()

	waitForSmokeAPI(t, apiAddress, &stdout, &stderr)
	client, err := Dial(context.Background(), apiAddress, 3*time.Second)
	if err != nil {
		t.Fatalf("dial xray API: %v", err)
	}
	defer client.Close()

	if tags, err := client.ListOutboundTags(context.Background()); err != nil {
		t.Fatalf("list outbounds: %v", err)
	} else if !contains(tags, "direct") {
		t.Fatalf("outbound tags = %v, want direct", tags)
	}
	if err := client.AddOutbound(context.Background(), mustOutbound(t, map[string]any{
		"tag":      "direct-smoke",
		"protocol": "freedom",
		"settings": map[string]any{},
	})); err != nil {
		t.Fatalf("add outbound: %v", err)
	}
	if tags, err := client.ListOutboundTags(context.Background()); err != nil {
		t.Fatalf("list outbounds after add: %v", err)
	} else {
		requireContains(t, tags, "direct-smoke")
	}
	if err := client.RemoveOutbound(context.Background(), "direct-smoke"); err != nil {
		t.Fatalf("remove outbound: %v", err)
	}
	if tags, err := client.ListOutboundTags(context.Background()); err != nil {
		t.Fatalf("list outbounds after remove: %v", err)
	} else {
		requireNotContains(t, tags, "direct-smoke")
	}
	if err := client.AddInboundUser(context.Background(), "trojan-smoke", "smoke@example.com", "secret"); err != nil {
		t.Fatalf("add inbound user: %v", err)
	}
	if users, err := client.ListInboundUserEmails(context.Background(), "trojan-smoke"); err != nil {
		t.Fatalf("list inbound users: %v", err)
	} else if !contains(users, "smoke@example.com") {
		t.Fatalf("inbound users = %v, want smoke@example.com", users)
	}
	if err := client.RemoveInboundUser(context.Background(), "trojan-smoke", "smoke@example.com"); err != nil {
		t.Fatalf("remove inbound user: %v", err)
	}
	if users, err := client.ListInboundUserEmails(context.Background(), "trojan-smoke"); err != nil {
		t.Fatalf("list inbound users after remove: %v", err)
	} else {
		requireNotContains(t, users, "smoke@example.com")
	}
	if err := client.AddRule(context.Background(), map[string]any{
		"type":        "field",
		"ruleTag":     "smoke-rule",
		"inboundTag":  []any{"socks-smoke"},
		"outboundTag": "direct",
	}); err != nil {
		t.Fatalf("add rule: %v", err)
	}
	if tags, err := client.ListRuleTags(context.Background()); err != nil {
		t.Fatalf("list rules after add: %v", err)
	} else {
		requireContains(t, tags, "smoke-rule")
	}
	route, err := client.TestRoute(context.Background(), RouteTest{
		InboundTag:     "socks-smoke",
		Network:        "tcp",
		TargetDomain:   "example.com",
		TargetPort:     443,
		FieldSelectors: []string{"outbound"},
	})
	if err != nil {
		t.Fatalf("test route: %v", err)
	}
	if route.OutboundTag != "direct" {
		t.Fatalf("route outbound = %q, want direct", route.OutboundTag)
	}
	if err := client.RemoveRule(context.Background(), "smoke-rule"); err != nil {
		t.Fatalf("remove rule: %v", err)
	}
	if tags, err := client.ListRuleTags(context.Background()); err != nil {
		t.Fatalf("list rules after remove: %v", err)
	} else {
		requireNotContains(t, tags, "smoke-rule")
	}
	if _, err := client.GetOutboundStatuses(context.Background()); err != nil {
		t.Fatalf("get outbound statuses: %v", err)
	}
	if _, err := QueryStats(context.Background(), StatsQueryOptions{Address: apiAddress, Pattern: "", Timeout: 3 * time.Second}); err != nil {
		t.Fatalf("query stats: %v", err)
	}
}

func bundledXrayPath(t *testing.T) string {
	t.Helper()
	if value := os.Getenv("XP2P_XRAY_BIN"); value != "" {
		return value
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	for _, candidate := range bundledXrayCandidates(root) {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	t.Fatalf("bundled xray binary not found under %s; set XP2P_XRAY_BIN", root)
	return ""
}

func bundledXrayCandidates(root string) []string {
	switch runtime.GOOS {
	case "windows":
		arch := "x86"
		buildArch := "386"
		if runtime.GOARCH == "amd64" {
			arch = "x86_64"
			buildArch = "amd64"
		}
		return []string{
			filepath.Join(root, "build", "msi-bin", "bundle", "xray.exe"),
			filepath.Join(root, "build", "windows-"+buildArch, "xray.exe"),
			filepath.Join(root, "distro", "windows", "bundle", arch, "xray.exe"),
		}
	case "linux":
		return []string{filepath.Join(root, "distro", "linux", "bundle", bundleArch(), "xray")}
	case "darwin":
		return []string{filepath.Join(root, "distro", "macos", "bundle", bundleArch(), "xray")}
	default:
		return nil
	}
}

func bundleArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "386":
		return "x86"
	case "arm":
		return "arm32"
	default:
		return runtime.GOARCH
	}
}

func freeTCPAddress(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	_, portRaw, err := net.SplitHostPort(freeTCPAddress(t))
	if err != nil {
		t.Fatalf("split free address: %v", err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatalf("parse free port: %v", err)
	}
	return port
}

func writeSmokeXrayConfig(t *testing.T, path, workDir, apiAddress string) {
	t.Helper()
	host, portRaw, err := net.SplitHostPort(apiAddress)
	if err != nil {
		t.Fatalf("split API address: %v", err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatalf("parse API port: %v", err)
	}
	doc := map[string]any{
		"log": map[string]any{
			"loglevel": "debug",
			"access":   filepath.Join(workDir, "access.log"),
			"error":    filepath.Join(workDir, "error.log"),
		},
		"api": map[string]any{
			"tag":      "api",
			"services": []string{"HandlerService", "RoutingService", "StatsService", "LoggerService", "ObservatoryService"},
		},
		"observatory": map[string]any{
			"subjectSelector": []string{"direct"},
			"probeURL":        "http://127.0.0.1:9",
			"probeInterval":   "10m",
		},
		"stats": map[string]any{},
		"policy": map[string]any{
			"levels": map[string]any{
				"0": map[string]any{
					"statsUserDownlink": true,
					"statsUserUplink":   true,
					"statsUserOnline":   true,
				},
			},
			"system": map[string]any{
				"statsInboundDownlink":  true,
				"statsInboundUplink":    true,
				"statsOutboundDownlink": true,
				"statsOutboundUplink":   true,
			},
		},
		"inbounds": []any{
			map[string]any{
				"tag":      "api",
				"listen":   host,
				"port":     port,
				"protocol": "dokodemo-door",
				"settings": map[string]any{"address": "127.0.0.1"},
			},
			map[string]any{
				"tag":      "socks-smoke",
				"listen":   "127.0.0.1",
				"port":     freeTCPPort(t),
				"protocol": "socks",
				"settings": map[string]any{"auth": "noauth", "udp": true},
			},
			map[string]any{
				"tag":      "trojan-smoke",
				"listen":   "127.0.0.1",
				"port":     freeTCPPort(t),
				"protocol": "trojan",
				"settings": map[string]any{"clients": []any{}},
				"streamSettings": map[string]any{
					"network":  "tcp",
					"security": "none",
					"tcpSettings": map[string]any{
						"header": map[string]any{"type": "none"},
					},
				},
			},
		},
		"outbounds": []any{
			map[string]any{
				"tag":      "direct",
				"protocol": "freedom",
				"settings": map[string]any{"domainStrategy": "UseIP"},
			},
		},
		"routing": map[string]any{
			"rules": []any{
				map[string]any{"type": "field", "inboundTag": []string{"api"}, "outboundTag": "api"},
				map[string]any{"type": "field", "inboundTag": []string{"socks-smoke"}, "outboundTag": "direct"},
			},
		},
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func waitForSmokeAPI(t *testing.T, address string, stdout, stderr *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for API at %s\nstdout:\n%s\nstderr:\n%s", address, stdout.String(), stderr.String())
}

func mustOutbound(t *testing.T, outbound map[string]any) *coreconfig.OutboundHandlerConfig {
	t.Helper()
	cfg, err := OutboundFromMap(outbound)
	if err != nil {
		t.Fatalf("convert outbound: %v", err)
	}
	return cfg
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func requireContains(t *testing.T, values []string, want string) {
	t.Helper()
	if !contains(values, want) {
		t.Fatalf("values = %v, want %q", values, want)
	}
}

func requireNotContains(t *testing.T, values []string, want string) {
	t.Helper()
	if contains(values, want) {
		t.Fatalf("values = %v, did not want %q", values, want)
	}
}
