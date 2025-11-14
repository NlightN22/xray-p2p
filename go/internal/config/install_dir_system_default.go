//go:build !linux

package config

func detectSystemInstallDir() string {
	return ""
}
