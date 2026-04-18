package clientcmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func ensureDeployTargetAvailable(cfg config.Config, opts deployOptions) error {
	host := strings.TrimSpace(opts.runtime.serverHost)
	if host == "" {
		host = strings.TrimSpace(opts.runtime.remoteHost)
	}
	if host == "" {
		return fmt.Errorf("deploy host is required")
	}
	portStr := strings.TrimSpace(opts.manifest.trojanPort)
	if portStr == "" {
		return nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("invalid trojan port %q", portStr)
	}

	records, err := clientListFunc(client.ListOptions{
		InstallDir: strings.TrimSpace(cfg.Client.InstallDir),
		ConfigDir:  strings.TrimSpace(cfg.Client.ConfigDir),
		Pending:    !clientLiveConfigPresent(),
	})
	if err != nil {
		return err
	}
	for _, record := range records {
		if strings.EqualFold(record.Hostname, host) && record.Port == port {
			return fmt.Errorf("endpoint %s:%d already exists", host, port)
		}
	}
	return nil
}

func clientLiveConfigPresent() bool {
	path := config.LiveConfigPath(layout.ClientConfigFileName)
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}
