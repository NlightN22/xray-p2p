package clientcmd

import (
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func defaultClientServiceLogPath(installDir string) string {
	return defaultClientLogPath(installDir, "service.log")
}

func defaultClientLogPath(installDir string, fileName string) string {
	return filepath.Join(config.LogRoot(), "client", fileName)
}
