package root

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestResolveSocksAddressExplicit(t *testing.T) {
	cfg := config.Config{}
	addr, err := resolveSocksAddress(cfg, "10.0.0.10:9000")
	if err != nil {
		t.Fatalf("resolveSocksAddress returned error: %v", err)
	}
	if addr != "10.0.0.10:9000" {
		t.Fatalf("unexpected socks address: %s", addr)
	}
}

func TestResolveSocksAddressAutoPrefersClient(t *testing.T) {
	cfg := config.Config{}
	tmp := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", tmp)
	cfg.Client.InstallDir = tmp
	cfg.Client.ConfigDir = "config-client"
	cfg.Server.InstallDir = tmp
	cfg.Server.ConfigDir = "config-server"

	writeSocksInbound(t, filepath.Join(tmp, ".state", "live", cfg.Client.ConfigDir), "127.0.0.1", 1111)
	writeSocksInbound(t, filepath.Join(tmp, ".state", "live", cfg.Server.ConfigDir), "127.0.0.1", 2222)

	addr, err := resolveSocksAddress(cfg, tunnelConfigSentinel)
	if err != nil {
		t.Fatalf("resolveSocksAddress returned error: %v", err)
	}
	if addr != "127.0.0.1:1111" {
		t.Fatalf("expected client socks address, got %s", addr)
	}
}

func TestResolveSocksAddressAutoFallsBackToServer(t *testing.T) {
	cfg := config.Config{}
	tmp := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", tmp)
	cfg.Client.InstallDir = tmp
	cfg.Client.ConfigDir = "config-client"
	cfg.Server.InstallDir = tmp
	cfg.Server.ConfigDir = "config-server"

	writeNonSocksInbound(t, filepath.Join(tmp, ".state", "live", cfg.Client.ConfigDir))
	writeSocksInbound(t, filepath.Join(tmp, ".state", "live", cfg.Server.ConfigDir), "127.0.0.1", 3333)

	addr, err := resolveSocksAddress(cfg, tunnelConfigSentinel)
	if err != nil {
		t.Fatalf("resolveSocksAddress returned error: %v", err)
	}
	if addr != "127.0.0.1:3333" {
		t.Fatalf("expected server socks address, got %s", addr)
	}
}

func TestResolveSocksAddressAutoMissing(t *testing.T) {
	cfg := config.Config{}
	cfg.Client.ConfigDir = "config-client"
	cfg.Server.ConfigDir = "config-server"

	_, err := resolveSocksAddress(cfg, tunnelConfigSentinel)
	if err == nil {
		t.Fatal("expected error when socks proxy cannot be detected")
	}
	want := "tunnel proxy not configured; specify --tunnel host:port or install xp2p client/server"
	if err.Error() != want {
		t.Fatalf("unexpected error %q (want %q)", err.Error(), want)
	}
}

func TestDetectSocksProxiesIgnoresDefaultClientWhenMissingConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", tmp)

	cfg := config.Config{}
	cfg.Client.InstallDir = tmp
	cfg.Client.ConfigDir = "config-client"
	cfg.Client.SocksAddress = "127.0.0.1:51180"
	cfg.Server.InstallDir = tmp
	cfg.Server.ConfigDir = "config-server"

	writeSocksInbound(t, filepath.Join(tmp, ".state", "live", cfg.Server.ConfigDir), "127.0.0.1", 51080)

	clientAddr, serverAddr, err := detectSocksProxies(cfg)
	if err != nil {
		t.Fatalf("detectSocksProxies returned error: %v", err)
	}
	if clientAddr != "" {
		t.Fatalf("expected empty client socks addr, got %q", clientAddr)
	}
	if serverAddr != "127.0.0.1:51080" {
		t.Fatalf("expected server socks addr, got %q", serverAddr)
	}
}

func TestResolveSocksAddressInvalid(t *testing.T) {
	cfg := config.Config{}
	if _, err := resolveSocksAddress(cfg, "bad-port"); err == nil {
		t.Fatal("expected error for invalid socks address")
	}
}

func writeSocksInbound(t *testing.T, dir, listen string, port int) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create dir %s: %v", dir, err)
	}
	doc := map[string]any{
		"inbounds": []map[string]any{
			{
				"protocol": "socks",
				"listen":   listen,
				"port":     port,
			},
		},
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal inbounds: %v", err)
	}
	path := filepath.Join(dir, "inbounds.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeNonSocksInbound(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create dir %s: %v", dir, err)
	}
	doc := map[string]any{
		"inbounds": []map[string]any{
			{
				"protocol": "dokodemo-door",
				"listen":   "0.0.0.0",
				"port":     9999,
			},
		},
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal inbounds: %v", err)
	}
	path := filepath.Join(dir, "inbounds.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
