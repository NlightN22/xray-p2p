//go:build !windows

package config

import (
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func defaultConfigRoot() string {
	return defaultInstallDir()
}

func defaultLogRoot() string {
	return filepath.Clean(layout.UnixLogRoot)
}
