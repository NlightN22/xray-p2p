//go:build windows

package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func defaultConfigRoot() string {
	return filepath.Join(programDataDir(), "xp2p")
}

func defaultLogRoot() string {
	return filepath.Join(defaultConfigRoot(), layout.LogsDirName)
}

func programDataDir() string {
	if value := strings.TrimSpace(os.Getenv("ProgramData")); value != "" {
		return value
	}
	if drive := strings.TrimSpace(os.Getenv("SystemDrive")); drive != "" {
		return filepath.Join(drive, "ProgramData")
	}
	return filepath.Join("C:\\", "ProgramData")
}
