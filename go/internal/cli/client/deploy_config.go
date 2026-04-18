package clientcmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func loadDeployClientConfig() (config.Config, error) {
	path := config.LiveConfigPath(layout.ClientConfigFileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			path = config.PendingConfigPath(layout.ClientConfigFileName)
		} else {
			return config.Config{}, err
		}
	}
	return config.Load(config.Options{
		Path:         path,
		AllowInvalid: true,
	})
}

func resolveDeployFullTunnelTag(installDir, configDir string, link trojanLink, runtime runtimeOptions) (string, error) {
	host := strings.TrimSpace(link.ServerAddress)
	if host == "" {
		host = strings.TrimSpace(runtime.serverHost)
	}
	if host == "" {
		host = strings.TrimSpace(runtime.remoteHost)
	}
	if host == "" {
		return "", fmt.Errorf("deploy host is required for full-tunnel")
	}

	records, err := clientListFunc(client.ListOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		Pending:    !clientLiveConfigPresent(),
	})
	if err != nil {
		return "", err
	}
	for _, record := range records {
		if strings.EqualFold(record.Hostname, host) && strings.TrimSpace(record.Tag) != "" {
			return record.Tag, nil
		}
	}
	return "", fmt.Errorf("full-tunnel endpoint %s not found", host)
}

func logDeployPaths(message, updatedPath string) {
	applyPath := config.ApplyRequestPath()
	applyDir := filepath.Dir(applyPath)
	logging.Info(
		message,
		"mode_config", updatedPath,
		"live_config", config.LiveConfigPath(layout.ClientConfigFileName),
		"pending_config", config.PendingConfigPath(layout.ClientConfigFileName),
		"apply_dir", applyDir,
		"apply_request", applyPath,
		"live_exists", fileExists(config.LiveConfigPath(layout.ClientConfigFileName)),
		"pending_exists", fileExists(config.PendingConfigPath(layout.ClientConfigFileName)),
		"apply_dir_exists", dirExists(applyDir),
		"apply_request_exists", fileExists(applyPath),
	)
}
