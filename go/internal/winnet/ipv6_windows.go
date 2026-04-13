//go:build windows

package winnet

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const (
	ipv6ComponentID         = "ms_tcpip6"
	ipv6DisableWaitTimeout  = 20 * time.Second
	ipv6DisablePollInterval = 1 * time.Second
)

type ipv6DisableResult int

const (
	ipv6ResultNoChange ipv6DisableResult = iota
	ipv6ResultDisabled
	ipv6ResultMissing
)

// DisableIPv6BindingWithRetry disables IPv6 binding on the named adapter after it appears.
func DisableIPv6BindingWithRetry(ctx context.Context, adapterName string) {
	name := strings.TrimSpace(adapterName)
	if name == "" {
		return
	}

	deadline := time.Now().Add(ipv6DisableWaitTimeout)
	for {
		if ctx.Err() != nil {
			return
		}
		result, err := disableIPv6BindingOnce(ctx, name)
		if err != nil {
			logging.Warn("failed to disable IPv6 binding", "interface", name, "err", err)
			return
		}
		switch result {
		case ipv6ResultMissing:
			if time.Now().After(deadline) {
				logging.Warn("IPv6 binding disable skipped (interface not found)", "interface", name)
				return
			}
			time.Sleep(ipv6DisablePollInterval)
		case ipv6ResultDisabled:
			logging.Info("IPv6 binding disabled", "interface", name, "component", ipv6ComponentID)
			return
		default:
			logging.Info("IPv6 binding already disabled", "interface", name, "component", ipv6ComponentID)
			return
		}
	}
}

func disableIPv6BindingOnce(ctx context.Context, adapterName string) (ipv6DisableResult, error) {
	result, err := disableIPv6BindingWithPowerShell(ctx, adapterName)
	if err == nil {
		return result, nil
	}
	if errors.Is(err, errPowerShellUnsupported) {
		return disableIPv6BindingWithNetsh(ctx, adapterName)
	}
	return ipv6ResultNoChange, err
}

func runPowerShell(ctx context.Context, script string) (string, error) {
	psPath, err := lookPathSystem32("powershell.exe")
	if err != nil {
		return "", errPowerShellNotFound
	}
	cmd := exec.CommandContext(ctx, psPath, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xp2p: powershell failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func escapePowerShellString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

var (
	errPowerShellNotFound    = errors.New("xp2p: powershell.exe not found")
	errPowerShellUnsupported = errors.New("xp2p: NetAdapterBinding cmdlets unavailable")
)

func disableIPv6BindingWithPowerShell(ctx context.Context, adapterName string) (ipv6DisableResult, error) {
	escaped := escapePowerShellString(adapterName)
	script := strings.Join([]string{
		`$cmd = Get-Command Get-NetAdapterBinding -ErrorAction SilentlyContinue`,
		`if ($null -eq $cmd) { Write-Output "unsupported"; exit 0 }`,
		`$binding = Get-NetAdapterBinding -Name '` + escaped + `' -ComponentID '` + ipv6ComponentID + `' -ErrorAction SilentlyContinue | Select-Object -First 1`,
		`if ($null -eq $binding) { Write-Output "missing"; exit 0 }`,
		`if ($binding.Enabled) {`,
		`  Disable-NetAdapterBinding -Name '` + escaped + `' -ComponentID '` + ipv6ComponentID + `' -Confirm:$false -ErrorAction Stop | Out-Null`,
		`  Write-Output "disabled"`,
		`} else { Write-Output "noop" }`,
	}, "; ")

	out, err := runPowerShell(ctx, script)
	if err != nil {
		if errors.Is(err, errPowerShellNotFound) {
			return ipv6ResultNoChange, errPowerShellUnsupported
		}
		return ipv6ResultNoChange, err
	}

	switch strings.ToLower(strings.TrimSpace(out)) {
	case "unsupported":
		return ipv6ResultNoChange, errPowerShellUnsupported
	case "missing":
		return ipv6ResultMissing, nil
	case "disabled":
		return ipv6ResultDisabled, nil
	case "noop":
		return ipv6ResultNoChange, nil
	default:
		return ipv6ResultNoChange, fmt.Errorf("xp2p: unexpected PowerShell output: %q", out)
	}
}

func disableIPv6BindingWithNetsh(ctx context.Context, adapterName string) (ipv6DisableResult, error) {
	state, err := getNetshIPv6InterfaceState(ctx, adapterName)
	if err != nil {
		return ipv6ResultNoChange, err
	}
	if state == "" {
		return ipv6ResultMissing, nil
	}
	if strings.EqualFold(state, "disabled") {
		return ipv6ResultNoChange, nil
	}
	if err := runNetsh(ctx, adapterName); err != nil {
		return ipv6ResultNoChange, err
	}
	return ipv6ResultDisabled, nil
}

func runNetsh(ctx context.Context, adapterName string) error {
	netshPath, err := lookPathSystem32("netsh.exe")
	if err != nil {
		return fmt.Errorf("xp2p: netsh.exe not found: %w", err)
	}
	cmd := exec.CommandContext(ctx, netshPath, "interface", "ipv6", "set", "interface", fmt.Sprintf("interface=%s", adapterName), "disabled")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("xp2p: netsh failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func getNetshIPv6InterfaceState(ctx context.Context, adapterName string) (string, error) {
	netshPath, err := lookPathSystem32("netsh.exe")
	if err != nil {
		return "", fmt.Errorf("xp2p: netsh.exe not found: %w", err)
	}
	cmd := exec.CommandContext(ctx, netshPath, "interface", "ipv6", "show", "interface")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xp2p: netsh show interface failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	target := strings.ToLower(strings.TrimSpace(adapterName))
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if _, err := strconv.Atoi(fields[0]); err != nil {
			continue
		}
		name := strings.ToLower(strings.Join(fields[4:], " "))
		if name == target {
			return fields[3], nil
		}
	}
	return "", nil
}

func lookPathSystem32(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	roots := []string{
		os.Getenv("SystemRoot"),
		os.Getenv("WINDIR"),
	}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		candidates := []string{
			filepath.Join(root, "System32", name),
		}
		if os.Getenv("PROCESSOR_ARCHITEW6432") != "" {
			candidates = append(candidates, filepath.Join(root, "Sysnative", name))
		}
		if strings.EqualFold(name, "powershell.exe") {
			candidates = append(candidates, filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", name))
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("xp2p: %s not found", name)
}
