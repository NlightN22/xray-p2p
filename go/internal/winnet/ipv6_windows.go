//go:build windows

package winnet

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
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
			logging.Warn("xp2p: failed to disable IPv6 binding", "interface", name, "err", err)
			return
		}
		switch result {
		case ipv6ResultMissing:
			if time.Now().After(deadline) {
				logging.Warn("xp2p: IPv6 binding disable skipped (interface not found)", "interface", name)
				return
			}
			time.Sleep(ipv6DisablePollInterval)
		case ipv6ResultDisabled:
			logging.Info("xp2p: IPv6 binding disabled", "interface", name, "component", ipv6ComponentID)
			return
		default:
			logging.Info("xp2p: IPv6 binding already disabled", "interface", name, "component", ipv6ComponentID)
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
	psPath, err := exec.LookPath("powershell.exe")
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
	exists, err := adapterExistsWMIC(ctx, adapterName)
	if err != nil {
		return ipv6ResultNoChange, err
	}
	if !exists {
		return ipv6ResultMissing, nil
	}
	if err := runNetsh(ctx, adapterName); err != nil {
		return ipv6ResultNoChange, err
	}
	return ipv6ResultDisabled, nil
}

func adapterExistsWMIC(ctx context.Context, adapterName string) (bool, error) {
	wmicPath, err := exec.LookPath("wmic.exe")
	if err != nil {
		return false, fmt.Errorf("xp2p: wmic.exe not found: %w", err)
	}
	escaped := escapeWmiString(adapterName)
	query := fmt.Sprintf("NetConnectionID='%s'", escaped)
	cmd := exec.CommandContext(ctx, wmicPath, "path", "Win32_NetworkAdapter", "where", query, "get", "NetConnectionID", "/value")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("xp2p: wmic failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "NetConnectionID=") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "NetConnectionID="))
			return value != "", nil
		}
	}
	return false, nil
}

func runNetsh(ctx context.Context, adapterName string) error {
	netshPath, err := exec.LookPath("netsh.exe")
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

func escapeWmiString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
