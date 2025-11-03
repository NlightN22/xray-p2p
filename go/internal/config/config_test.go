package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if !cfg.Client.AllowInsecure {
		t.Fatalf("expected default client allowInsecure to be true")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := chdirTemp(t)

	writeFile(t, filepath.Join(dir, "xp2p.yaml"), `
logging:
  level: warn
  format: json
server:
  port: 65001
  install_dir: C:\xp2p-test
  config_dir: cfg-test
  mode: manual
  certificate: C:\certs\server.pem
  key: C:\certs\server.key
  host: server.example.test
client:
  install_dir: D:\xp2p-client
  config_dir: cfg-client
  server_address: remote.example.com
  server_port: 9343
  user: client@example.com
  password: strongpass
  server_name: sni.example.com
  allow_insecure: false
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
}

func TestLoadFromEnv(t *testing.T) {
	chdirTemp(t)

	t.Setenv("XP2P_LOGGING_LEVEL", "DEBUG")
	t.Setenv("XP2P_LOGGING_FORMAT", "JSON")
	t.Setenv("XP2P_SERVER_PORT", "65002")
	t.Setenv("XP2P_SERVER_INSTALL_DIR", `D:\xp2p`)
	t.Setenv("XP2P_SERVER_CONFIG_DIR", "cfg-dir")
	t.Setenv("XP2P_SERVER_MODE", "AUTO")
	t.Setenv("XP2P_SERVER_CERTIFICATE", `D:\certs\cert.pem`)
	t.Setenv("XP2P_SERVER_KEY", `D:\certs\cert.key`)
	t.Setenv("XP2P_CLIENT_INSTALL_DIR", `E:\xp2p-client`)
	t.Setenv("XP2P_CLIENT_CONFIG_DIR", "cfg-client")
	t.Setenv("XP2P_CLIENT_SERVER_ADDRESS", "remote.env")
	t.Setenv("XP2P_CLIENT_SERVER_PORT", "9543")
	t.Setenv("XP2P_CLIENT_USER", "env@example.com")
	t.Setenv("XP2P_CLIENT_PASSWORD", "envpass")
	t.Setenv("XP2P_CLIENT_SERVER_NAME", "env.example.com")
	t.Setenv("XP2P_CLIENT_ALLOW_INSECURE", "false")

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
}

func TestLoadOverrides(t *testing.T) {
	chdirTemp(t)

	t.Setenv("XP2P_LOGGING_LEVEL", "debug")

	cfg, err := Load(Options{
		Overrides: map[string]any{
			"logging.level":         "error",
			"logging.format":        "json",
			"server.port":           "65003",
			"server.install_dir":    `E:\xp2p`,
			"server.config_dir":     "cfg-override",
			"server.mode":           "MANUAL",
			"server.certificate":    `E:\certs\cert.pem`,
			"server.key":            `E:\certs\cert.key`,
			"client.install_dir":    `F:\xp2p-client`,
			"client.config_dir":     "cfg-client-override",
			"client.server_address": "remote.override",
			"client.server_port":    "9643",
			"client.user":           "override@example.com",
			"client.password":       "overridepass",
			"client.server_name":    "override.example.com",
			"client.allow_insecure": false,
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
}

func TestLoadWithExplicitPath(t *testing.T) {
	dir := chdirTemp(t)

	cfgPath := filepath.Join(dir, "custom.toml")
	writeFile(t, cfgPath, `
[logging]
level = "warn"
format = "json"

[server]
port = "65004"
install_dir = "C:\\xp2p-custom"
config_dir = "config-alt"
mode = "Manual"
certificate = "C:\\certs\\server.pem"
key = "C:\\certs\\server.key"
host = "custom.example.test"

[client]
install_dir = "D:\\xp2p-client"
config_dir = "cfg-client"
server_address = "remote.toml"
server_port = "9743"
user = "client.toml@example.com"
password = "tomlpass"
server_name = "toml.example.com"
allow_insecure = false
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
}

func TestLoadInvalidPath(t *testing.T) {
	dir := chdirTemp(t)

	_, err := Load(Options{Path: dir})
	if err == nil {
		t.Fatalf("expected error for directory path")
	}
}

func chdirTemp(t *testing.T) string {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	return tmp
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
