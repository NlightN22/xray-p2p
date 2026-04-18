package config

import "testing"

func TestLoadOverrides(t *testing.T) {
	chdirTemp(t)

	t.Setenv("XP2P_LOGGING_LEVEL", "debug")

	cfg, err := Load(Options{
		Overrides: map[string]any{
			"logging.level":              "error",
			"logging.format":             "json",
			"server.port":                "65003",
			"server.trojan_port":         "58446",
			"server.install_dir":         `E:\xp2p`,
			"server.config_dir":          "cfg-override",
			"server.mode":                "MANUAL",
			"server.certificate":         `E:\certs\cert.pem`,
			"server.key":                 `E:\certs\cert.key`,
			"server.tun_enabled":         false,
			"server.tun_name":            "server-override",
			"server.tun_mtu":             1420,
			"server.tun_addr":            "198.18.0.25/30",
			"client.install_dir":         `F:\xp2p-client`,
			"client.config_dir":          "cfg-client-override",
			"client.server_address":      "remote.override",
			"client.server_port":         "9643",
			"client.user":                "override@example.com",
			"client.password":            "overridepass",
			"client.server_name":         "override.example.com",
			"client.allow_insecure":      false,
			"client.tun_enabled":         true,
			"client.tun_name":            "client-override",
			"client.tun_mtu":             1320,
			"client.tun_addr":            "198.18.0.29/30",
			"client.tun_mode":            "full",
			"client.dns_servers":         []string{"1.1.1.1", "8.8.8.8"},
			"client.full_tunnel_verbose": true,
			"client.full_tunnel_tag":     "proxy-override",
		},
	})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "error" {
		t.Fatalf("expected error level, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected json format, got %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "65003" {
		t.Fatalf("expected port 65003, got %s", cfg.Server.Port)
	}
	if cfg.Server.TrojanPort != "58446" {
		t.Fatalf("expected trojan port 58446, got %s", cfg.Server.TrojanPort)
	}
	if cfg.Server.InstallDir != `E:\xp2p` {
		t.Fatalf("expected install dir E:\\xp2p, got %s", cfg.Server.InstallDir)
	}
	if cfg.Server.ConfigDir != "cfg-override" {
		t.Fatalf("expected config dir cfg-override, got %s", cfg.Server.ConfigDir)
	}
	if cfg.Server.Mode != "manual" {
		t.Fatalf("expected mode manual, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateFile != `E:\certs\cert.pem` {
		t.Fatalf("expected certificate E:\\certs\\cert.pem, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != `E:\certs\cert.key` {
		t.Fatalf("expected key E:\\certs\\cert.key, got %s", cfg.Server.KeyFile)
	}
	if cfg.Server.TunEnabled {
		t.Fatalf("expected server tun enabled false from overrides")
	}
	if cfg.Server.TunName != "server-override" {
		t.Fatalf("expected server tun name server-override, got %s", cfg.Server.TunName)
	}
	if cfg.Server.TunMTU != 1420 {
		t.Fatalf("expected server tun MTU 1420, got %d", cfg.Server.TunMTU)
	}
	if cfg.Server.TunAddr != "198.18.0.25/30" {
		t.Fatalf("expected server tun addr 198.18.0.25/30, got %s", cfg.Server.TunAddr)
	}
	if cfg.Client.InstallDir != `F:\xp2p-client` {
		t.Fatalf("expected client install dir F:\\xp2p-client, got %s", cfg.Client.InstallDir)
	}
	if cfg.Client.ConfigDir != "cfg-client-override" {
		t.Fatalf("expected client config dir cfg-client-override, got %s", cfg.Client.ConfigDir)
	}
	if cfg.Client.ServerAddress != "remote.override" {
		t.Fatalf("expected client server address remote.override, got %s", cfg.Client.ServerAddress)
	}
	if cfg.Client.ServerPort != "9643" {
		t.Fatalf("expected client server port 9643, got %s", cfg.Client.ServerPort)
	}
	if cfg.Client.User != "override@example.com" {
		t.Fatalf("expected client user override@example.com, got %s", cfg.Client.User)
	}
	if cfg.Client.Password != "overridepass" {
		t.Fatalf("expected client password overridepass, got %s", cfg.Client.Password)
	}
	if cfg.Client.ServerName != "override.example.com" {
		t.Fatalf("expected client server name override.example.com, got %s", cfg.Client.ServerName)
	}
	if cfg.Client.AllowInsecure {
		t.Fatalf("expected client allowInsecure false from overrides")
	}
	if !cfg.Client.TunEnabled {
		t.Fatalf("expected client tun enabled true from overrides")
	}
	if cfg.Client.TunName != "client-override" {
		t.Fatalf("expected client tun name client-override, got %s", cfg.Client.TunName)
	}
	if cfg.Client.TunMTU != 1320 {
		t.Fatalf("expected client tun MTU 1320, got %d", cfg.Client.TunMTU)
	}
	if cfg.Client.TunAddr != "198.18.0.29/30" {
		t.Fatalf("expected client tun addr 198.18.0.29/30, got %s", cfg.Client.TunAddr)
	}
	if cfg.Client.TunMode != "full" {
		t.Fatalf("expected client tun mode full, got %s", cfg.Client.TunMode)
	}
	if len(cfg.Client.DNSServers) != 2 || cfg.Client.DNSServers[0] != "1.1.1.1" || cfg.Client.DNSServers[1] != "8.8.8.8" {
		t.Fatalf("unexpected client dns servers: %v", cfg.Client.DNSServers)
	}
	if !cfg.Client.FullTunnelVerbose {
		t.Fatalf("expected full tunnel verbose true from overrides")
	}
	if cfg.Client.FullTunnelTag != "proxy-override" {
		t.Fatalf("expected full tunnel tag proxy-override, got %s", cfg.Client.FullTunnelTag)
	}
}
