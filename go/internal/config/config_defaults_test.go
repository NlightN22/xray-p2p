package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	chdirTemp(t)

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "info" {
		t.Fatalf("unexpected logging level: %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Fatalf("unexpected logging format: %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "62022" {
		t.Fatalf("unexpected server port: %s", cfg.Server.Port)
	}
	if cfg.Server.TrojanPort != "58443" {
		t.Fatalf("unexpected server trojan port: %s", cfg.Server.TrojanPort)
	}
	if cfg.Server.Profile != "trojan-tls" {
		t.Fatalf("unexpected server profile: %s", cfg.Server.Profile)
	}
	if cfg.Server.InstallDir == "" {
		t.Fatalf("expected non-empty install dir")
	}
	if cfg.Server.ConfigDir != "config-server" {
		t.Fatalf("expected default config dir config-server, got %s", cfg.Server.ConfigDir)
	}
	if cfg.Server.Mode != "auto" {
		t.Fatalf("expected mode auto, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateFile != "" {
		t.Fatalf("expected empty certificate path, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != "" {
		t.Fatalf("expected empty key path, got %s", cfg.Server.KeyFile)
	}
	if cfg.Server.Host != "" {
		t.Fatalf("expected empty server host by default, got %s", cfg.Server.Host)
	}
	if cfg.Server.TunEnabled {
		t.Fatalf("expected server tun disabled by default")
	}
	if cfg.Server.TunName != "xp2ps" {
		t.Fatalf("expected server tun name xp2ps, got %s", cfg.Server.TunName)
	}
	if cfg.Server.TunMTU != 1500 {
		t.Fatalf("expected server tun MTU 1500, got %d", cfg.Server.TunMTU)
	}
	if cfg.Server.TunAddr != "198.18.0.5/30" {
		t.Fatalf("expected server tun addr 198.18.0.5/30, got %s", cfg.Server.TunAddr)
	}
	if cfg.Client.InstallDir == "" {
		t.Fatalf("expected non-empty client install dir")
	}
	if cfg.Client.ConfigDir != "config-client" {
		t.Fatalf("expected default client config dir config-client, got %s", cfg.Client.ConfigDir)
	}
	if cfg.Client.ServerAddress != "" {
		t.Fatalf("expected empty client server address by default, got %s", cfg.Client.ServerAddress)
	}
	if cfg.Client.ServerPort != "8443" {
		t.Fatalf("expected default client server port 8443, got %s", cfg.Client.ServerPort)
	}
	if cfg.Client.User != "" {
		t.Fatalf("expected empty client user by default")
	}
	if cfg.Client.Password != "" {
		t.Fatalf("expected empty client password by default")
	}
	if cfg.Client.ServerName != "" {
		t.Fatalf("expected empty client server name by default")
	}
	if cfg.Client.AllowInsecure {
		t.Fatalf("expected default client allowInsecure to be false")
	}
	if !cfg.Client.TunEnabled {
		t.Fatalf("expected client tun enabled by default")
	}
	if cfg.Client.TunName != "xp2pc" {
		t.Fatalf("expected client tun name xp2pc, got %s", cfg.Client.TunName)
	}
	if cfg.Client.TunMTU != 1500 {
		t.Fatalf("expected client tun MTU 1500, got %d", cfg.Client.TunMTU)
	}
	if cfg.Client.TunAddr != "198.18.0.1/30" {
		t.Fatalf("expected client tun addr 198.18.0.1/30, got %s", cfg.Client.TunAddr)
	}
	if cfg.Client.TunMode != "split" {
		t.Fatalf("expected client tun mode split, got %s", cfg.Client.TunMode)
	}
	if len(cfg.Client.DNSServers) != 0 {
		t.Fatalf("expected empty client dns servers by default")
	}
	if cfg.Client.FullTunnelVerbose {
		t.Fatalf("expected full tunnel verbose disabled by default")
	}
	if cfg.Client.FullTunnelTag != "" {
		t.Fatalf("expected empty full tunnel tag by default")
	}
}
