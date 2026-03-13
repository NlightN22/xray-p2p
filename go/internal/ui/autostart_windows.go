//go:build windows

package ui

import (
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const autoStartKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

func EnsureAutoStart(enabled bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	appName := "xp2p-ui"
	return setRunKey(appName, exe, enabled)
}

func setRunKey(name, exe string, enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, autoStartKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if !enabled {
		if err := key.DeleteValue(name); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}

	value := quotePath(exe)
	return key.SetStringValue(name, value)
}

func quotePath(path string) string {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return ""
	}
	if strings.HasPrefix(clean, "\"") && strings.HasSuffix(clean, "\"") {
		return clean
	}
	return "\"" + clean + "\""
}
