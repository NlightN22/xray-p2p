//go:build windows

package winnet

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

var ErrTunIPv4TentativeTimeout = errors.New("tun IPv4 remained tentative")

const tunTentativeTimeout = 10 * time.Second

func WaitForTunIPv4(ctx context.Context, tunName string, tunAddr string, verbose bool) (int, string, error) {
	name := strings.TrimSpace(tunName)
	addr := strings.TrimSpace(tunAddr)
	if name == "" && addr == "" {
		return 0, "", errors.New("tun name or addr required for IPv4 wait")
	}
	deadline := time.Now().Add(10 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		deadline = ctxDeadline
	}
	attempt := 0
	var lastErr error
	var tentativeSince time.Time
	for {
		attempt++
		ifIndex, err := resolveTunInterface(ctx, name, addr)
		if err == nil && ifIndex > 0 {
			details, stateErr := InterfaceIPv4Details(ifIndex)
			if stateErr != nil && !errors.Is(stateErr, ErrTunIPv4Missing) {
				return 0, "", stateErr
			}
			ip, oper, dad, ready := statusFromIPv4Details(details)
			if ip != "" && details.DadState == dadStateTentative {
				if tentativeSince.IsZero() {
					tentativeSince = time.Now()
				}
				if time.Since(tentativeSince) >= tunTentativeTimeout {
					logging.Warn(
						"tun IPv4 remained tentative",
						"ifIndex", ifIndex,
						"ip", ip,
						"operStatus", oper,
						"dadState", dad,
					)
					return 0, "", ErrTunIPv4TentativeTimeout
				}
			} else {
				tentativeSince = time.Time{}
			}
			if ready {
				logging.Info(
					"tun IPv4 ready",
					"ifIndex", ifIndex,
					"ip", ip,
					"operStatus", oper,
					"dadState", dad,
				)
				if verbose {
					logging.Info("tun IPv4 ready details", "ifIndex", ifIndex, "attempt", attempt)
				}
				return ifIndex, ip, nil
			}
			lastErr = ErrTunIPv4Missing
			if verbose {
				logging.Info("waiting for tun IPv4", "ifIndex", ifIndex, "operStatus", oper, "dadState", dad, "attempt", attempt)
			}
		} else if err != nil && !errors.Is(err, ErrInterfaceNotFound) {
			return 0, "", err
		} else {
			lastErr = ErrInterfaceNotFound
			if verbose {
				logging.Info("waiting for tun interface", "interface", name, "attempt", attempt)
			}
		}
		if ctx.Err() != nil {
			return 0, "", ctx.Err()
		}
		if time.Now().After(deadline) {
			if lastErr == nil {
				lastErr = ErrInterfaceNotFound
			}
			return 0, "", lastErr
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func resolveTunInterface(ctx context.Context, tunName string, tunAddr string) (int, error) {
	if tunAddr != "" {
		ifIndex, err := InterfaceIndexByIP(tunAddr)
		if err == nil && ifIndex > 0 {
			return ifIndex, nil
		}
		if err != nil && !errors.Is(err, ErrInterfaceNotFound) {
			return 0, err
		}
	}
	if tunName == "" {
		return 0, ErrInterfaceNotFound
	}
	ifIndex, _, _, err := InterfaceByNamePrefix(tunName)
	if err == nil && ifIndex > 0 {
		return ifIndex, nil
	}
	ifIndex, err = InterfaceIndexByName(ctx, tunName)
	if err == nil && ifIndex > 0 {
		return ifIndex, nil
	}
	if errors.Is(err, ErrInterfaceNotFound) {
		ifIndex, _, _, err = InterfaceByDescriptionContains([]string{"xray tunnel", "wintun"})
		if err == nil && ifIndex > 0 {
			return ifIndex, nil
		}
	}
	if err != nil {
		return 0, err
	}
	return 0, ErrInterfaceNotFound
}

func EnsureTunIPv4(ctx context.Context, tunName string, tunAddr string, verbose bool) (int, string, error) {
	name := strings.TrimSpace(tunName)
	addr := strings.TrimSpace(tunAddr)
	if name == "" && addr == "" {
		return 0, "", errors.New("tun name or addr required for IPv4 ensure")
	}
	assignIP, assignPrefix, assignOK := parseTunAddr(addr)
	deadline := time.Now().Add(10 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		deadline = ctxDeadline
	}
	attempt := 0
	var lastErr error
	var tentativeSince time.Time
	for {
		attempt++
		ifIndex, err := resolveTunInterface(ctx, name, addr)
		if err == nil && ifIndex > 0 {
			details, stateErr := InterfaceIPv4Details(ifIndex)
			if stateErr != nil && !errors.Is(stateErr, ErrTunIPv4Missing) {
				return 0, "", stateErr
			}
			ip, oper, dad, ready := statusFromIPv4Details(details)
			if ip != "" && details.DadState == dadStateTentative {
				if tentativeSince.IsZero() {
					tentativeSince = time.Now()
				}
				if time.Since(tentativeSince) >= tunTentativeTimeout {
					logging.Warn(
						"tun IPv4 remained tentative",
						"ifIndex", ifIndex,
						"ip", ip,
						"operStatus", oper,
						"dadState", dad,
					)
					return 0, "", ErrTunIPv4TentativeTimeout
				}
			} else {
				tentativeSince = time.Time{}
			}
			shouldAssign := assignOK && (ip == "" || !strings.EqualFold(ip, assignIP))
			if assignOK {
				if shouldAssign {
					if err := ensureInterfaceIPv4(ctx, ifIndex, assignIP, assignPrefix); err != nil {
						lastErr = err
						if verbose {
							logging.Info("tun IPv4 assign attempt failed", "ifIndex", ifIndex, "attempt", attempt, "err", err)
						}
					}
				}
			}
			if ready {
				logging.Info(
					"tun IPv4 ready",
					"ifIndex", ifIndex,
					"ip", ip,
					"operStatus", oper,
					"dadState", dad,
				)
				if verbose {
					logging.Info("tun IPv4 ready details", "ifIndex", ifIndex, "attempt", attempt)
				}
				return ifIndex, ip, nil
			}
			lastErr = ErrTunIPv4Missing
			if verbose {
				logging.Info("waiting for tun IPv4", "ifIndex", ifIndex, "operStatus", oper, "dadState", dad, "attempt", attempt)
			}
		} else if err != nil && !errors.Is(err, ErrInterfaceNotFound) {
			return 0, "", err
		} else {
			lastErr = ErrInterfaceNotFound
			if verbose {
				logging.Info("waiting for tun interface", "interface", name, "attempt", attempt)
			}
		}
		if ctx.Err() != nil {
			return 0, "", ctx.Err()
		}
		if time.Now().After(deadline) {
			if lastErr == nil {
				lastErr = ErrInterfaceNotFound
			}
			return 0, "", lastErr
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func ensureInterfaceIPv4(ctx context.Context, ifIndex int, ip string, prefix int) error {
	if ifIndex <= 0 {
		return ErrInterfaceNotFound
	}
	if strings.TrimSpace(ip) == "" || prefix <= 0 {
		return ErrTunIPv4Missing
	}
	if err := assignInterfaceIPv4Native(ifIndex, ip, prefix); err == nil {
		logging.Info("tun IPv4 assigned", "ifIndex", ifIndex, "ip", ip, "prefix", prefix, "method", "native")
		return nil
	} else if !isUnicastIPHelperUnsupported(err) {
		logging.Warn("native tun IPv4 assign failed; falling back to PowerShell", "ifIndex", ifIndex, "ip", ip, "prefix", prefix, "err", err)
	} else {
		logging.Warn("native tun IPv4 assign unsupported; falling back to PowerShell", "ifIndex", ifIndex, "ip", ip, "prefix", prefix, "err", err)
	}
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$ifIndex = ` + strconv.Itoa(ifIndex),
		`$targetIp = '` + escapePowerShellString(ip) + `'`,
		`$prefix = ` + strconv.Itoa(prefix),
		`$existing = Get-NetIPAddress -AddressFamily IPv4 -InterfaceIndex $ifIndex -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -eq $targetIp } | Select-Object -First 1`,
		`if ($null -ne $existing) { Write-Output "ok"; exit 0 }`,
		`New-NetIPAddress -InterfaceIndex $ifIndex -IPAddress $targetIp -PrefixLength $prefix -PolicyStore ActiveStore -ErrorAction Stop | Out-Null`,
		`Write-Output "ok"`,
	}, "; ")
	_, err := runPowerShell(ctx, script)
	if err == nil {
		logging.Info("tun IPv4 assigned", "ifIndex", ifIndex, "ip", ip, "prefix", prefix, "method", "powershell")
	}
	return err
}

func interfaceIPv4ReadyState(ifIndex int) (string, string, string, bool, error) {
	details, err := InterfaceIPv4Details(ifIndex)
	if err != nil {
		return "", InterfaceOperStatusName(details.OperStatus), InterfaceDadStateName(details.DadState), false, err
	}
	ip, oper, dad, ready := statusFromIPv4Details(details)
	return ip, oper, dad, ready, nil
}

func statusFromIPv4Details(details IPv4Details) (string, string, string, bool) {
	oper := InterfaceOperStatusName(details.OperStatus)
	dad := InterfaceDadStateName(details.DadState)
	ready := details.IP != "" && details.DadState == dadStatePreferred
	return details.IP, oper, dad, ready
}
