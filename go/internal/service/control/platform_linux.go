//go:build linux

package control

import (
	"os"
	"strings"
)

func isOpenWrtSystem() bool {
	if hasFile("/etc/openwrt_release") || hasFile("/etc/openwrt_version") {
		return true
	}

	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "openwrt")
}

func hasFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
