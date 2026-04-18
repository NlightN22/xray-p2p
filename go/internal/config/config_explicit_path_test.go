package config

import (
	"path/filepath"
	"testing"
)

func TestLoadWithExplicitPath(t *testing.T) {
	dir := chdirTemp(t)

	cfgPath := filepath.Join(dir, "custom.toml")
	writeFile(t, cfgPath, `
[logging]
level = "warn"
format = "json"

[server]
port = "65004"
trojan_port = "58447"
install_dir = "C:\\xp2p-custom"
config_dir = "config-alt"
mode = "Manual"
certificate = "C:\\certs\\server.pem"
key = "C:\\certs\\server.key"
host = "custom.example.test"
tun_enabled = false
tun_name = "server-toml"
tun_mtu = 1410
tun_addr = "198.18.0.33/30"

[client]
install_dir = "D:\\xp2p-client"
config_dir = "cfg-client"
server_address = "remote.toml"
server_port = "9743"
user = "client.toml@example.com"
password = "tomlpass"
server_name = "toml.example.com"
allow_insecure = false
tun_enabled = true
tun_name = "client-toml"
tun_mtu = 1310
tun_addr = "198.18.0.37/30"
tun_mode = "full"
dns_servers = ["9.9.9.9"]
full_tunnel_verbose = true
full_tunnel_tag = "proxy-toml"
`)

	cfg, err := Load(Options{Path: cfgPath})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("expected warn level, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected json format, got %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "65004" {
		t.Fatalf("expected port 65004, got %s", cfg.Server.Port)
	}
	if cfg.Server.TrojanPort != "58447" {
		t.Fatalf("expected trojan port 58447, got %s", cfg.Server.TrojanPort)
	}
	if cfg.Server.InstallDir != `C:\xp2p-custom` {
		t.Fatalf("expected install dir C:\\xp2p-custom, got %s", cfg.Server.InstallDir)
	}
	if cfg.Server.ConfigDir != "config-alt" {
		t.Fatalf("expected config dir config-alt, got %s", cfg.Server.ConfigDir)
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
	if cfg.Server.Host != "custom.example.test" {
		t.Fatalf("expected server host custom.example.test, got %s", cfg.Server.Host)
	}
	if cfg.Server.TunEnabled {
		t.Fatalf("expected server tun enabled false from toml")
	}
	if cfg.Server.TunName != "server-toml" {
		t.Fatalf("expected server tun name server-toml, got %s", cfg.Server.TunName)
	}
	if cfg.Server.TunMTU != 1410 {
		t.Fatalf("expected server tun MTU 1410, got %d", cfg.Server.TunMTU)
	}
	if cfg.Server.TunAddr != "198.18.0.33/30" {
		t.Fatalf("expected server tun addr 198.18.0.33/30, got %s", cfg.Server.TunAddr)
	}
	if cfg.Client.InstallDir != `D:\xp2p-client` {
		t.Fatalf("expected client install dir D:\\xp2p-client, got %s", cfg.Client.InstallDir)
	}
	if cfg.Client.ConfigDir != "cfg-client" {
		t.Fatalf("expected client config dir cfg-client, got %s", cfg.Client.ConfigDir)
	}
	if cfg.Client.ServerAddress != "remote.toml" {
		t.Fatalf("expected client server address remote.toml, got %s", cfg.Client.ServerAddress)
	}
	if cfg.Client.ServerPort != "9743" {
		t.Fatalf("expected client server port 9743, got %s", cfg.Client.ServerPort)
	}
	if cfg.Client.User != "client.toml@example.com" {
		t.Fatalf("expected client user client.toml@example.com, got %s", cfg.Client.User)
	}
	if cfg.Client.Password != "tomlpass" {
		t.Fatalf("expected client password tomlpass, got %s", cfg.Client.Password)
	}
	if cfg.Client.ServerName != "toml.example.com" {
		t.Fatalf("expected client server name toml.example.com, got %s", cfg.Client.ServerName)
	}
	if cfg.Client.AllowInsecure {
		t.Fatalf("expected client allowInsecure false from file")
	}
	if !cfg.Client.TunEnabled {
		t.Fatalf("expected client tun enabled true from toml")
	}
	if cfg.Client.TunName != "client-toml" {
		t.Fatalf("expected client tun name client-toml, got %s", cfg.Client.TunName)
	}
	if cfg.Client.TunMTU != 1310 {
		t.Fatalf("expected client tun MTU 1310, got %d", cfg.Client.TunMTU)
	}
	if cfg.Client.TunAddr != "198.18.0.37/30" {
		t.Fatalf("expected client tun addr 198.18.0.37/30, got %s", cfg.Client.TunAddr)
	}
	if cfg.Client.TunMode != "full" {
		t.Fatalf("expected client tun mode full, got %s", cfg.Client.TunMode)
	}
	if len(cfg.Client.DNSServers) != 1 || cfg.Client.DNSServers[0] != "9.9.9.9" {
		t.Fatalf("unexpected client dns servers: %v", cfg.Client.DNSServers)
	}
	if !cfg.Client.FullTunnelVerbose {
		t.Fatalf("expected full tunnel verbose true from toml")
	}
	if cfg.Client.FullTunnelTag != "proxy-toml" {
		t.Fatalf("expected full tunnel tag proxy-toml, got %s", cfg.Client.FullTunnelTag)
	}
}
