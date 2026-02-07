//go:build linux

package linuxnet

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const managedMarker = "xp2p-managed"

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

	if err := writeNetworkdConfig(name, addr, mtu); err != nil {
		return err
	}
	if err := reloadNetworkd(); err != nil {
		logging.Warn("xp2p: systemd-networkd reload failed", "err", err)
	}

	logging.Info("xp2p: Linux networkd config ensured", "interface", name, "addr", addr)
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

	if err := removeNetworkdConfig(name); err != nil {
		return err
	}
	if err := reloadNetworkd(); err != nil {
		logging.Warn("xp2p: systemd-networkd reload failed", "err", err)
	}
	logging.Info("xp2p: Linux networkd config removed", "interface", name)
	return nil
}

func writeNetworkdConfig(name, addr string, mtu int) error {
	dir := "/etc/systemd/network"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("xp2p: create networkd dir: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("90-%s.network", name))
	table := tableForName(name)
	content := buildNetworkdConfig(name, addr, mtu, table)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("xp2p: write networkd config: %w", err)
	}
	return nil
}

func removeNetworkdConfig(name string) error {
	path := filepath.Join("/etc/systemd/network", fmt.Sprintf("90-%s.network", name))
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("xp2p: read networkd config: %w", err)
	}
	if !strings.Contains(string(data), managedMarker) {
		logging.Info("xp2p: networkd config not managed; skipping cleanup", "path", path)
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: remove networkd config: %w", err)
	}
	return nil
}

func buildNetworkdConfig(name, addr string, mtu int, table int) string {
	builder := strings.Builder{}
	builder.WriteString("# ")
	builder.WriteString(managedMarker)
	builder.WriteString("\n")
	builder.WriteString("[Match]\n")
	builder.WriteString("Name = ")
	builder.WriteString(name)
	builder.WriteString("\n\n[Network]\n")
	builder.WriteString("KeepConfiguration = yes\n")
	builder.WriteString("Address = ")
	builder.WriteString(addr)
	builder.WriteString("\n\n[Link]\n")
	if mtu > 0 {
		builder.WriteString("MTUBytes = ")
		builder.WriteString(fmt.Sprintf("%d", mtu))
		builder.WriteString("\n")
	}
	builder.WriteString("ActivationPolicy = manual\n")
	builder.WriteString("RequiredForOnline = no\n\n")
	builder.WriteString("[Route]\n")
	builder.WriteString("Table = ")
	builder.WriteString(fmt.Sprintf("%d", table))
	builder.WriteString("\n")
	builder.WriteString("Destination = 0.0.0.0/0\n")
	return builder.String()
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

func reloadNetworkd() error {
	if _, err := execLookPath("systemctl"); err != nil {
		return nil
	}
	return runCommand("systemctl", "reload", "systemd-networkd")
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
