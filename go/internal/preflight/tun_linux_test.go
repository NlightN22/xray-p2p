//go:build linux

package preflight

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestLinuxTunPreflightOpenWrtHint(t *testing.T) {
	oldOpen := osOpenFileFunc
	oldOpenWrt := isOpenWrtSystem
	t.Cleanup(func() {
		osOpenFileFunc = oldOpen
		isOpenWrtSystem = oldOpenWrt
	})

	isOpenWrtSystem = func() bool { return true }
	osOpenFileFunc = func(string, int, os.FileMode) (*os.File, error) {
		return nil, os.ErrNotExist
	}

	err := linuxTunPreflight{}.Check(context.Background(), TunConfig{Enabled: true})
	var tunErr ErrTunUnavailable
	if !errors.As(err, &tunErr) {
		t.Fatalf("expected ErrTunUnavailable, got %T (%v)", err, err)
	}
	if !strings.Contains(strings.ToLower(tunErr.Hint), "opkg") || !strings.Contains(strings.ToLower(tunErr.Hint), "kmod-tun") {
		t.Fatalf("unexpected hint: %q", tunErr.Hint)
	}
}

func TestLinuxTunPreflightPermissionHint(t *testing.T) {
	oldOpen := osOpenFileFunc
	oldOpenWrt := isOpenWrtSystem
	t.Cleanup(func() {
		osOpenFileFunc = oldOpen
		isOpenWrtSystem = oldOpenWrt
	})

	isOpenWrtSystem = func() bool { return false }
	osOpenFileFunc = func(string, int, os.FileMode) (*os.File, error) {
		return nil, os.ErrPermission
	}

	err := linuxTunPreflight{}.Check(context.Background(), TunConfig{Enabled: true})
	var tunErr ErrTunUnavailable
	if !errors.As(err, &tunErr) {
		t.Fatalf("expected ErrTunUnavailable, got %T (%v)", err, err)
	}
	if !strings.Contains(strings.ToLower(tunErr.Hint), "cap_net_admin") && !strings.Contains(strings.ToLower(tunErr.Hint), "root") {
		t.Fatalf("unexpected hint: %q", tunErr.Hint)
	}
}
