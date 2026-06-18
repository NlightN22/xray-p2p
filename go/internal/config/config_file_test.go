package config

import (
	"path/filepath"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	dir := chdirTemp(t)

	writeFile(t, filepath.Join(dir, "xp2p-client.toml"), `
[client]
install_dir = "D:\\xp2p-client"
config_dir = "cfg-client"
server_address = "remote.example.com"
server_port = "9343"
user = "client@example.com"
password = "strongpass"
server_name = "sni.example.com"
allow_insecure = false
tun_enabled = true
tun_name = "client-tun"
tun_mtu = 1300
tun_addr = "198.18.0.13/30"
tun_mode = "full"
dns_servers = ["1.1.1.1", "8.8.8.8"]
full_tunnel_verbose = true
full_tunnel_tag = "proxy-1"

[xray_assets]
stale_after = "72h"

[[xray_assets.files]]
name = "geoip.dat"
url = "https://example.test/geoip.dat"
`)
	writeFile(t, filepath.Join(dir, "xp2p-server.toml"), `
[logging]
level = "warn"
format = "json"

[server]
port = "65001"
trojan_port = "58444"
install_dir = "C:\\xp2p-test"
config_dir = "cfg-test"
mode = "manual"
certificate = "C:\\certs\\server.pem"
key = "C:\\certs\\server.key"
host = "server.example.test"
tun_enabled = false
tun_name = "server-tun"
tun_mtu = 1400
tun_addr = "198.18.0.9/30"
`)

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("expected warn level, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected json format, got %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "65001" {
		t.Fatalf("expected port 65001, got %s", cfg.Server.Port)
	}
	if cfg.Server.TrojanPort != "58444" {
		t.Fatalf("expected trojan port 58444, got %s", cfg.Server.TrojanPort)
	}
	if cfg.Server.InstallDir != `C:\xp2p-test` {
		t.Fatalf("expected install dir C:\\xp2p-test, got %s", cfg.Server.InstallDir)
	}
	if cfg.Server.ConfigDir != "cfg-test" {
		t.Fatalf("expected config dir cfg-test, got %s", cfg.Server.ConfigDir)
	}
	if cfg.Server.Mode != "manual" {
		t.Fatalf("expected mode manual, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateFile != `C:\certs\server.pem` {
		t.Fatalf("expected certificate C:\\certs\\server.pem, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != `C:\certs\server.key` {
		t.Fatalf("expected key C:\\certs\\server.key, got %s", cfg.Server.KeyFile)
	}
	if cfg.Server.Host != "server.example.test" {
		t.Fatalf("expected server host server.example.test, got %s", cfg.Server.Host)
	}
	if cfg.Server.TunEnabled {
		t.Fatalf("expected server tun enabled false from file")
	}
	if cfg.Server.TunName != "server-tun" {
		t.Fatalf("expected server tun name server-tun, got %s", cfg.Server.TunName)
	}
	if cfg.Server.TunMTU != 1400 {
		t.Fatalf("expected server tun MTU 1400, got %d", cfg.Server.TunMTU)
	}
	if cfg.Server.TunAddr != "198.18.0.9/30" {
		t.Fatalf("expected server tun addr 198.18.0.9/30, got %s", cfg.Server.TunAddr)
	}
	if cfg.Client.InstallDir != `D:\xp2p-client` {
		t.Fatalf("expected client install dir D:\\xp2p-client, got %s", cfg.Client.InstallDir)
	}
	if cfg.Client.ConfigDir != "cfg-client" {
		t.Fatalf("expected client config dir cfg-client, got %s", cfg.Client.ConfigDir)
	}
	if cfg.Client.ServerAddress != "remote.example.com" {
		t.Fatalf("expected client server address remote.example.com, got %s", cfg.Client.ServerAddress)
	}
	if cfg.Client.ServerPort != "9343" {
		t.Fatalf("expected client server port 9343, got %s", cfg.Client.ServerPort)
	}
	if cfg.Client.User != "client@example.com" {
		t.Fatalf("expected client user client@example.com, got %s", cfg.Client.User)
	}
	if cfg.Client.Password != "strongpass" {
		t.Fatalf("expected client password strongpass, got %s", cfg.Client.Password)
	}
	if cfg.Client.ServerName != "sni.example.com" {
		t.Fatalf("expected client server name sni.example.com, got %s", cfg.Client.ServerName)
	}
	if cfg.Client.AllowInsecure {
		t.Fatalf("expected client allowInsecure false from file")
	}
	if !cfg.Client.TunEnabled {
		t.Fatalf("expected client tun enabled true from file")
	}
	if cfg.Client.TunName != "client-tun" {
		t.Fatalf("expected client tun name client-tun, got %s", cfg.Client.TunName)
	}
	if cfg.Client.TunMTU != 1300 {
		t.Fatalf("expected client tun MTU 1300, got %d", cfg.Client.TunMTU)
	}
	if cfg.Client.TunAddr != "198.18.0.13/30" {
		t.Fatalf("expected client tun addr 198.18.0.13/30, got %s", cfg.Client.TunAddr)
	}
	if cfg.Client.TunMode != "full" {
		t.Fatalf("expected client tun mode full, got %s", cfg.Client.TunMode)
	}
	if len(cfg.Client.DNSServers) != 2 || cfg.Client.DNSServers[0] != "1.1.1.1" || cfg.Client.DNSServers[1] != "8.8.8.8" {
		t.Fatalf("unexpected client dns servers: %v", cfg.Client.DNSServers)
	}
	if !cfg.Client.FullTunnelVerbose {
		t.Fatalf("expected full tunnel verbose true from file")
	}
	if cfg.Client.FullTunnelTag != "proxy-1" {
		t.Fatalf("expected full tunnel tag proxy-1, got %s", cfg.Client.FullTunnelTag)
	}
	if cfg.XrayAssets.StaleAfter != "72h" {
		t.Fatalf("expected xray assets stale_after 72h, got %s", cfg.XrayAssets.StaleAfter)
	}
	if len(cfg.XrayAssets.Files) != 1 || cfg.XrayAssets.Files[0].Name != "geoip.dat" || cfg.XrayAssets.Files[0].URL == "" {
		t.Fatalf("unexpected xray assets files: %+v", cfg.XrayAssets.Files)
	}
}
