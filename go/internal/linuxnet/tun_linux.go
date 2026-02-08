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
		return errors.New("xp2p: tun name is required for Linux setup")
	}
	if addr == "" {
		return errors.New("xp2p: tun address is required for Linux setup")
	}
	if isOpenWrtSystem() {
		return nil
	}
	return nil
}

func RemoveTunInterfaceIfManaged(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if isOpenWrtSystem() {
		return nil
	}
	if err := removeTunRouting(name); err != nil {
		return err
	}
	return nil
}

func EnsureTunAddress(name, addr string, mtu int) error {
	name = strings.TrimSpace(name)
	addr = strings.TrimSpace(addr)
	if name == "" {
		return errors.New("xp2p: tun name is required for Linux setup")
	}
	if addr == "" {
		return errors.New("xp2p: tun address is required for Linux setup")
	}
	if isOpenWrtSystem() {
		return nil
	}
	if _, err := execLookPath("ip"); err != nil {
		return errors.New("xp2p: ip command not found")
	}

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if !linkExists(name) {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if addrPresent(name, addr) {
			return ensureTunRouting(name)
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
		return ensureTunRouting(name)
	}
	return fmt.Errorf("xp2p: tun interface %s not found", name)
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
		return fmt.Errorf("xp2p: %s %s: %v (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(buf.String()))
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

func ensureTunRouting(name string) error {
	table := tableForName(name)
	if err := ensureRouteTable(table, name); err != nil {
		return err
	}
	return ensureRuleTable(table)
}

func ensureRouteTable(table int, name string) error {
	return runCommand(
		"ip",
		"route",
		"replace",
		"default",
		"dev",
		name,
		"table",
		fmt.Sprintf("%d", table),
	)
}

func ensureRuleTable(table int) error {
	err := runCommand(
		"ip",
		"rule",
		"add",
		"pref",
		fmt.Sprintf("%d", table),
		"table",
		fmt.Sprintf("%d", table),
	)
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "File exists") {
		return nil
	}
	return err
}

func removeTunRouting(name string) error {
	table := tableForName(name)
	_ = runCommand(
		"ip",
		"route",
		"flush",
		"table",
		fmt.Sprintf("%d", table),
	)
	err := runCommand(
		"ip",
		"rule",
		"del",
		"pref",
		fmt.Sprintf("%d", table),
		"table",
		fmt.Sprintf("%d", table),
	)
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "No such file") {
		return nil
	}
	return err
}
