package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

var (
	defaultInstallDirOnce sync.Once
	defaultInstallDirPath string
)

func defaultInstallDir() string {
	defaultInstallDirOnce.Do(func() {
		defaultInstallDirPath = computeDefaultInstallDir()
	})
	return defaultInstallDirPath
}

func computeDefaultInstallDir() string {
	if dir := detectSelfInstallDir(); dir != "" {
		return dir
	}
	if dir := detectSystemInstallDir(); dir != "" {
		return dir
	}
	if dir := osPreferredInstallDir(); dir != "" {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "xp2p")
	}
	return filepath.Join(os.TempDir(), "xp2p")
}

func detectSelfInstallDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exePath)
	if dir == "" {
		return ""
	}
	if looksLikeInstallRoot(dir) {
		return dir
	}
	return ""
}

func looksLikeInstallRoot(dir string) bool {
	markers := []struct {
		path       string
		expectDir  bool
		expectFile bool
	}{
		{path: filepath.Join(dir, layout.StateFileName), expectFile: true},
		{path: filepath.Join(dir, layout.ClientStateFileName), expectFile: true},
		{path: filepath.Join(dir, layout.ServerStateFileName), expectFile: true},
		{path: filepath.Join(dir, layout.ClientConfigDir), expectDir: true},
		{path: filepath.Join(dir, layout.ServerConfigDir), expectDir: true},
	}
	for _, marker := range markers {
		info, err := os.Stat(marker.path)
		if err != nil {
			continue
		}
		if marker.expectDir && info.IsDir() {
			return true
		}
		if marker.expectFile && !info.IsDir() {
			return true
		}
	}

	binDir := filepath.Join(dir, layout.BinDirName)
	logsDir := filepath.Join(dir, layout.LogsDirName)
	if info, err := os.Stat(binDir); err == nil && info.IsDir() {
		if logInfo, err := os.Stat(logsDir); err == nil && logInfo.IsDir() {
			return true
		}
	}
	return false
}
