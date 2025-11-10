//go:build windows

package config

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func osPreferredInstallDir() string {
	programFiles := programFilesDir()
	candidate := filepath.Join(programFiles, "xp2p")
	if dirExists(candidate) && dirWritable(candidate) {
		return candidate
	}
	if userIsAdministrator() {
		return candidate
	}

	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		return filepath.Join(local, "xp2p")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "AppData", "Local", "xp2p")
	}
	return candidate
}

func programFilesDir() string {
	envKeys := []string{"ProgramW6432", "ProgramFiles"}
	for _, key := range envKeys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	if drive := strings.TrimSpace(os.Getenv("SystemDrive")); drive != "" {
		return filepath.Join(drive, "Program Files")
	}
	return filepath.Join("C:\\", "Program Files")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func dirWritable(path string) bool {
	file, err := os.CreateTemp(path, ".xp2p-perm-*")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true
}

func userIsAdministrator() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()

	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false
	}
	member, err := token.IsMember(adminSID)
	if err != nil {
		return false
	}
	return member
}
