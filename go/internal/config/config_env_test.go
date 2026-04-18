package config

import "testing"

func TestLoadFromEnv(t *testing.T) {
	chdirTemp(t)

	t.Setenv("XP2P_LOGGING_LEVEL", "DEBUG")
	t.Setenv("XP2P_LOGGING_FORMAT", "JSON")
	t.Setenv("XP2P_SERVER_PORT", "65002")
	t.Setenv("XP2P_SERVER_TROJAN_PORT", "58445")
	t.Setenv("XP2P_SERVER_INSTALL_DIR", `D:\xp2p`)
	t.Setenv("XP2P_SERVER_CONFIG_DIR", "cfg-dir")
	t.Setenv("XP2P_SERVER_MODE", "AUTO")
	t.Setenv("XP2P_SERVER_CERTIFICATE", `D:\certs\cert.pem`)
	t.Setenv("XP2P_SERVER_KEY", `D:\certs\cert.key`)
	t.Setenv("XP2P_SERVER_TUN_ENABLED", "false")
	t.Setenv("XP2P_SERVER_TUN_NAME", "server-env")
	t.Setenv("XP2P_SERVER_TUN_MTU", "1450")
	t.Setenv("XP2P_SERVER_TUN_ADDR", "198.18.0.17/30")
	t.Setenv("XP2P_CLIENT_INSTALL_DIR", `E:\xp2p-client`)
	t.Setenv("XP2P_CLIENT_CONFIG_DIR", "cfg-client")
	t.Setenv("XP2P_CLIENT_SERVER_ADDRESS", "remote.env")
	t.Setenv("XP2P_CLIENT_SERVER_PORT", "9543")
	t.Setenv("XP2P_CLIENT_USER", "env@example.com")
	t.Setenv("XP2P_CLIENT_PASSWORD", "envpass")
	t.Setenv("XP2P_CLIENT_SERVER_NAME", "env.example.com")
	t.Setenv("XP2P_CLIENT_ALLOW_INSECURE", "false")
	t.Setenv("XP2P_CLIENT_TUN_ENABLED", "true")
	t.Setenv("XP2P_CLIENT_TUN_NAME", "client-env")
	t.Setenv("XP2P_CLIENT_TUN_MTU", "1350")
	t.Setenv("XP2P_CLIENT_TUN_ADDR", "198.18.0.21/30")
	t.Setenv("XP2P_CLIENT_TUN_MODE", "FULL")
	t.Setenv("XP2P_CLIENT_FULL_TUNNEL_VERBOSE", "true")
	t.Setenv("XP2P_CLIENT_FULL_TUNNEL_TAG", "proxy-env")

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected debug level, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected json format, got %s", cfg.Logging.Format)
	}
	if cfg.Server.Port != "65002" {
		t.Fatalf("expected port 65002, got %s", cfg.Server.Port)
	}
	if cfg.Server.TrojanPort != "58445" {
		t.Fatalf("expected trojan port 58445, got %s", cfg.Server.TrojanPort)
	}
	if cfg.Server.InstallDir != `D:\xp2p` {
		t.Fatalf("expected install dir D:\\xp2p, got %s", cfg.Server.InstallDir)
	}
	if cfg.Server.ConfigDir != "cfg-dir" {
		t.Fatalf("expected config dir cfg-dir, got %s", cfg.Server.ConfigDir)
	}
	if cfg.Server.Mode != "auto" {
		t.Fatalf("expected normalized mode auto, got %s", cfg.Server.Mode)
	}
	if cfg.Server.CertificateFile != `D:\certs\cert.pem` {
		t.Fatalf("expected certificate D:\\certs\\cert.pem, got %s", cfg.Server.CertificateFile)
	}
	if cfg.Server.KeyFile != `D:\certs\cert.key` {
		t.Fatalf("expected key D:\\certs\\cert.key, got %s", cfg.Server.KeyFile)
	}
	if cfg.Server.TunEnabled {
		t.Fatalf("expected server tun enabled false from env")
	}
	if cfg.Server.TunName != "server-env" {
		t.Fatalf("expected server tun name server-env, got %s", cfg.Server.TunName)
	}
	if cfg.Server.TunMTU != 1450 {
		t.Fatalf("expected server tun MTU 1450, got %d", cfg.Server.TunMTU)
	}
	if cfg.Server.TunAddr != "198.18.0.17/30" {
		t.Fatalf("expected server tun addr 198.18.0.17/30, got %s", cfg.Server.TunAddr)
	}
	if cfg.Client.InstallDir != `E:\xp2p-client` {
		t.Fatalf("expected client install dir E:\\xp2p-client, got %s", cfg.Client.InstallDir)
	}
	if cfg.Client.ConfigDir != "cfg-client" {
		t.Fatalf("expected client config dir cfg-client, got %s", cfg.Client.ConfigDir)
	}
	if cfg.Client.ServerAddress != "remote.env" {
		t.Fatalf("expected client server address remote.env, got %s", cfg.Client.ServerAddress)
	}
	if cfg.Client.ServerPort != "9543" {
		t.Fatalf("expected client server port 9543, got %s", cfg.Client.ServerPort)
	}
	if cfg.Client.User != "env@example.com" {
		t.Fatalf("expected client user env@example.com, got %s", cfg.Client.User)
	}
	if cfg.Client.Password != "envpass" {
		t.Fatalf("expected client password envpass, got %s", cfg.Client.Password)
	}
	if cfg.Client.ServerName != "env.example.com" {
		t.Fatalf("expected client server name env.example.com, got %s", cfg.Client.ServerName)
	}
	if cfg.Client.AllowInsecure {
		t.Fatalf("expected client allowInsecure false from env")
	}
	if !cfg.Client.TunEnabled {
		t.Fatalf("expected client tun enabled true from env")
	}
	if cfg.Client.TunName != "client-env" {
		t.Fatalf("expected client tun name client-env, got %s", cfg.Client.TunName)
	}
	if cfg.Client.TunMTU != 1350 {
		t.Fatalf("expected client tun MTU 1350, got %d", cfg.Client.TunMTU)
	}
	if cfg.Client.TunAddr != "198.18.0.21/30" {
		t.Fatalf("expected client tun addr 198.18.0.21/30, got %s", cfg.Client.TunAddr)
	}
	if cfg.Client.TunMode != "full" {
		t.Fatalf("expected client tun mode full, got %s", cfg.Client.TunMode)
	}
	if !cfg.Client.FullTunnelVerbose {
		t.Fatalf("expected full tunnel verbose true from env")
	}
	if cfg.Client.FullTunnelTag != "proxy-env" {
		t.Fatalf("expected full tunnel tag proxy-env, got %s", cfg.Client.FullTunnelTag)
	}
}
