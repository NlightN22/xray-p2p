package servercmd

import (
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func defaultServerServiceLogPath(installDir string) string {
	return defaultServerLogPath(installDir, "service.log")
}

func defaultServerLogPath(installDir string, fileName string) string {
	return filepath.Join(config.LogRoot(), "server", fileName)
}
