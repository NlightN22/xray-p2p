package config

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
)

// UpdateServerTrojanPort updates server.trojan_port in the specified config file and returns the path used.
func UpdateServerTrojanPort(path string, port string) (string, error) {
	return updateServerTrojanPort(path, port, configio.WriteOptions{
		AuditPath: AuditLogPath(),
	})
}

// UpdateServerTrojanPortBestEffort updates server.trojan_port and ignores audit log errors.
func UpdateServerTrojanPortBestEffort(path string, port string) (string, error) {
	return updateServerTrojanPort(path, port, configio.WriteOptions{
		AuditPath:         AuditLogPath(),
		IgnoreAuditErrors: true,
	})
}

func updateServerTrojanPort(path string, port string, opts configio.WriteOptions) (string, error) {
	port = strings.TrimSpace(port)
	if port == "" {
		return "", fmt.Errorf("config: server trojan port is required")
	}
	portValue, err := strconv.Atoi(port)
	if err != nil || portValue <= 0 || portValue > 65535 {
		return "", fmt.Errorf("config: invalid server trojan port %q", port)
	}

	configPath, err := resolveConfigPath(path, "server")
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(configPath)) != ".toml" {
		return "", fmt.Errorf("config: only toml files are supported for server trojan port updates")
	}

	tree, err := loadOrCreateToml(configPath)
	if err != nil {
		return "", err
	}
	tree.SetPath([]string{"server", "trojan_port"}, port)

	data, err := encodeToml(tree)
	if err != nil {
		return "", err
	}
	if err := configio.WriteBytes(configPath, data, opts); err != nil {
		return "", err
	}
	return configPath, nil
}

// ClearServerCertificateOverrides resets server certificate fields in the specified config file.
func ClearServerCertificateOverrides(path string) (string, error) {
	configPath, err := resolveConfigPath(path, "server")
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(configPath)) != ".toml" {
		return "", fmt.Errorf("config: only toml files are supported for server certificate updates")
	}

	tree, err := loadOrCreateToml(configPath)
	if err != nil {
		return "", err
	}
	tree.SetPath([]string{"server", "cert_store"}, "")
	tree.SetPath([]string{"server", "certificate"}, "")
	tree.SetPath([]string{"server", "key"}, "")

	data, err := encodeToml(tree)
	if err != nil {
		return "", err
	}
	if err := configio.WriteBytes(configPath, data, configio.WriteOptions{
		AuditPath: AuditLogPath(),
	}); err != nil {
		return "", err
	}
	return configPath, nil
}
