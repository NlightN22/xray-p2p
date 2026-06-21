//go:build linux

package linuxnet

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func EnsureTunInterface(name, addr string, mtu int) error {
	name = strings.TrimSpace(name)
	addr = strings.TrimSpace(addr)
	if name == "" {
		return errors.New("tun name is required for Linux setup")
	}
	if addr == "" {
		return errors.New("tun address is required for Linux setup")
	}
	if isOpenWrtSystem() {
		return nil
	}
	if _, err := execLookPath("ip"); err != nil {
		return errors.New("ip command not found")
	}
	if !linkExists(name) {
		if err := runCommand("ip", "tuntap", "add", "dev", name, "mode", "tun"); err != nil {
			return err
		}
	}
	return EnsureTunAddress(name, addr, mtu)
}

func RemoveTunInterfaceIfManaged(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if isOpenWrtSystem() {
		return nil
	}
	if !linkExists(name) {
		return nil
	}
	return runCommand("ip", "link", "del", name)
}

func RemoveTunInterfacesExcept(activeName string, names ...string) error {
	activeName = strings.TrimSpace(activeName)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || strings.EqualFold(name, activeName) {
			continue
		}
		if err := RemoveTunInterfaceIfManaged(name); err != nil {
			return err
		}
	}
	return nil
}

func EnsureTunAddress(name, addr string, mtu int) error {
	name = strings.TrimSpace(name)
	addr = strings.TrimSpace(addr)
	if name == "" {
		return errors.New("tun name is required for Linux setup")
	}
	if addr == "" {
		return errors.New("tun address is required for Linux setup")
	}
	if isOpenWrtSystem() {
		return nil
	}
	if _, err := execLookPath("ip"); err != nil {
		return errors.New("ip command not found")
	}

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if !linkExists(name) {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if addrPresent(name, addr) {
			return nil
		}
		if mtu > 0 {
			_ = runCommand("ip", "link", "set", "dev", name, "mtu", fmt.Sprintf("%d", mtu))
		}
		if err := runCommand("ip", "addr", "replace", addr, "dev", name); err != nil {
			return err
		}
		if err := runCommand("ip", "link", "set", "dev", name, "up"); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("tun interface %s not found", name)
}

func EnsureRoute(name, cidr string) error {
	name = strings.TrimSpace(name)
	cidr = strings.TrimSpace(cidr)
	if name == "" || cidr == "" {
		return nil
	}
	if isOpenWrtSystem() {
		return nil
	}
	if _, err := execLookPath("ip"); err != nil {
		return errors.New("ip command not found")
	}
	return runCommand("ip", "route", "replace", cidr, "dev", name)
}

func RemoveRoute(name, cidr string) error {
	name = strings.TrimSpace(name)
	cidr = strings.TrimSpace(cidr)
	if name == "" || cidr == "" {
		return nil
	}
	if isOpenWrtSystem() {
		return nil
	}
	if _, err := execLookPath("ip"); err != nil {
		return errors.New("ip command not found")
	}
	if err := runCommand("ip", "route", "del", cidr, "dev", name); err != nil {
		if isMissingRouteError(err) {
			return nil
		}
		return err
	}
	return nil
}

func tableForName(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "xp2ps":
		return 20091
	case "xp2pc":
		return 20090
	default:
		return 20090
	}
}

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

func execLookPath(file string) (string, error) {
	return execLookPathFunc(file)
}

var execLookPathFunc = func(file string) (string, error) { return exec.LookPath(file) }

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %v (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(buf.String()))
	}
	return nil
}

func linkExists(name string) bool {
	cmd := exec.Command("ip", "link", "show", "dev", name)
	return cmd.Run() == nil
}

func addrPresent(name, addr string) bool {
	output, err := captureCommand("ip", "-4", "addr", "show", "dev", name)
	if err != nil {
		return false
	}
	return strings.Contains(output, addr)
}

func captureCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(buf.String()), err
	}
	return strings.TrimSpace(buf.String()), nil
}

func isMissingRouteError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "no such process") ||
		strings.Contains(lower, "not found") ||
		strings.Contains(lower, "can't find device") ||
		strings.Contains(lower, "cannot find device")
}
