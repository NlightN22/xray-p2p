//go:build !windows && !linux

package config

func osPreferredInstallDir() string {
	return ""
}
