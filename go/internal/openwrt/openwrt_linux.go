//go:build linux

package openwrt

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type uciSection struct {
	name    string
	kind    string
	options map[string][]string
}

func (s uciSection) option(name string) string {
	values := s.options[name]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func EnsureTunInterface(name, addr string) error {
	name = strings.TrimSpace(name)
	addr = strings.TrimSpace(addr)
	if name == "" {
		return errors.New("tun name is required for OpenWrt setup")
	}
	if addr == "" {
		return errors.New("tun address is required for OpenWrt setup")
	}
	if !isOpenWrtSystem() {
		return nil
	}
	if _, err := exec.LookPath("uci"); err != nil {
		return errors.New("uci command not found (OpenWrt required)")
	}

	managed, exists, err := isManagedInterface(name)
	if err != nil {
		return err
	}
	if exists && !managed {
		logging.Warn("OpenWrt interface exists and is not managed; overwriting", "interface", name)
	}

	if exists {
		if err := runCommand("uci", "-q", "delete", "network."+name); err != nil {
			return err
		}
	}
	if err := runCommand("uci", "set", fmt.Sprintf("network.%s=interface", name)); err != nil {
		return err
	}
	if err := runCommand("uci", "set", fmt.Sprintf("network.%s.device=%s", name, name)); err != nil {
		return err
	}
	if err := runCommand("uci", "set", fmt.Sprintf("network.%s.proto=static", name)); err != nil {
		return err
	}
	if err := runCommand("uci", "add_list", fmt.Sprintf("network.%s.ipaddr=%s", name, addr)); err != nil {
		return err
	}
	if err := runCommand("uci", "set", fmt.Sprintf("network.%s.xp2p_managed=1", name)); err != nil {
		return err
	}
	if err := runCommand("uci", "commit", "network"); err != nil {
		return err
	}
	if err := runCommand("/etc/init.d/network", "reload"); err != nil {
		return err
	}

	logging.Info("OpenWrt TUN interface configured", "interface", name, "addr", addr)
	return nil
}

func EnsureTunRoute(name, cidr string) error {
	name = strings.TrimSpace(name)
	cidr = strings.TrimSpace(cidr)
	if name == "" || cidr == "" {
		return nil
	}
	if !isOpenWrtSystem() {
		return nil
	}
	if _, err := exec.LookPath("ip"); err != nil {
		return errors.New("ip command not found (OpenWrt required)")
	}
	var lastErr error
	for attempt := 0; attempt < 17; attempt++ {
		if err := runCommand("ip", "route", "replace", cidr, "dev", name); err != nil {
			lastErr = err
			if isMissingDeviceError(err) {
				time.Sleep(300 * time.Millisecond)
				continue
			}
			return err
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return nil
}

func RemoveTunRoute(name, cidr string) error {
	name = strings.TrimSpace(name)
	cidr = strings.TrimSpace(cidr)
	if name == "" || cidr == "" {
		return nil
	}
	if !isOpenWrtSystem() {
		return nil
	}
	if _, err := exec.LookPath("ip"); err != nil {
		return errors.New("ip command not found (OpenWrt required)")
	}
	if err := runCommand("ip", "route", "del", cidr, "dev", name); err != nil {
		if isMissingRouteError(err) {
			return nil
		}
		return err
	}
	return nil
}

func RemoveTunInterfaceIfManaged(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if !isOpenWrtSystem() {
		return nil
	}
	if _, err := exec.LookPath("uci"); err != nil {
		return errors.New("uci command not found (OpenWrt required)")
	}

	managed, exists, err := isManagedInterface(name)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if !managed {
		logging.Info("OpenWrt interface not managed; skipping cleanup", "interface", name)
		return nil
	}

	if err := runCommand("uci", "-q", "delete", "network."+name); err != nil {
		return err
	}
	if err := runCommand("uci", "commit", "network"); err != nil {
		return err
	}
	if err := runCommand("/etc/init.d/network", "reload"); err != nil {
		return err
	}
	logging.Info("OpenWrt TUN interface removed", "interface", name)
	return nil
}

func isManagedInterface(name string) (bool, bool, error) {
	out, err := captureCommand("uci", "-q", "show", "network."+name)
	if strings.TrimSpace(out) == "" {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("uci show network.%s: %w", name, err)
	}
	sections := parseUCIShow(out)
	section, ok := sections[name]
	if !ok {
		return false, false, nil
	}
	val := strings.TrimSpace(section.option("xp2p_managed"))
	return val == "1", true, nil
}

func parseUCIShow(output string) map[string]uciSection {
	result := make(map[string]uciSection)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		chunks := strings.Split(key, ".")
		if len(chunks) < 2 {
			continue
		}
		sectionName := chunks[1]
		section := result[sectionName]
		section.name = sectionName
		if len(chunks) == 2 {
			section.kind = val
		} else {
			if section.options == nil {
				section.options = make(map[string][]string)
			}
			optionName := chunks[2]
			section.options[optionName] = parseUCIValues(val)
		}
		result[sectionName] = section
	}
	return result
}

func parseUCIValues(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	fields := strings.Fields(raw)
	values := make([]string, 0, len(fields))
	for _, f := range fields {
		values = append(values, strings.Trim(f, "\"'"))
	}
	return values
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

func captureCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
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

func isMissingDeviceError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "can't find device") || strings.Contains(lower, "cannot find device")
}
