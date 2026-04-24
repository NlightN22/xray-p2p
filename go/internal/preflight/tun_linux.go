//go:build linux

package preflight

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
)

type linuxTunPreflight struct{}

var (
	osOpenFileFunc  = os.OpenFile
	isOpenWrtSystem = isOpenWrt
)

func (linuxTunPreflight) Check(ctx context.Context, cfg TunConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}
	const path = "/dev/net/tun"
	f, err := osOpenFileFunc(path, os.O_RDWR, 0)
	if err == nil {
		_ = f.Close()
		return nil
	}

	hint := "enable the TUN kernel module (modprobe tun) and ensure /dev/net/tun is accessible"
	if isOpenWrtSystem() {
		hint = "install the tun module: opkg update && opkg install kmod-tun"
	}
	if errors.Is(err, os.ErrNotExist) {
		return ErrTunUnavailable{
			OS:     osLabel(),
			Reason: fmt.Sprintf("%s is missing", path),
			Hint:   hint,
		}
	}
	if errors.Is(err, os.ErrPermission) {
		return ErrTunUnavailable{
			OS:     osLabel(),
			Reason: fmt.Sprintf("%s is not accessible (%v)", path, err),
			Hint:   "run with sufficient privileges (root or CAP_NET_ADMIN) and ensure /dev/net/tun is accessible",
		}
	}
	return ErrTunUnavailable{
		OS:     osLabel(),
		Reason: fmt.Sprintf("open %s failed (%v)", path, err),
		Hint:   hint,
	}
}

func osLabel() string {
	if isOpenWrtSystem() {
		return "openwrt"
	}
	return runtime.GOOS
}

func isOpenWrt() bool {
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
