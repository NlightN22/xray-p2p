//go:build windows

package winnet

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func WaitForTunIPv4(ctx context.Context, tunName string, tunAddr string, verbose bool) (int, string, error) {
	name := strings.TrimSpace(tunName)
	addr := strings.TrimSpace(tunAddr)
	if name == "" && addr == "" {
		return 0, "", errors.New("xp2p: tun name or addr required for IPv4 wait")
	}
	deadline := time.Now().Add(10 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		deadline = ctxDeadline
	}
	attempt := 0
	var lastErr error
	for {
		attempt++
		ifIndex, err := resolveTunInterface(ctx, name, addr)
		if err == nil && ifIndex > 0 {
			ip, ipErr := InterfaceIPv4(ctx, ifIndex)
			if ipErr != nil {
				return 0, "", ipErr
			}
			if strings.TrimSpace(ip) != "" {
				if verbose {
					logging.Info("xp2p: tun IPv4 ready", "ifIndex", ifIndex, "ip", ip, "attempt", attempt)
				}
				return ifIndex, ip, nil
			}
			lastErr = ErrTunIPv4Missing
			if verbose {
				logging.Info("xp2p: waiting for tun IPv4", "ifIndex", ifIndex, "attempt", attempt)
			}
		} else if err != nil && !errors.Is(err, ErrInterfaceNotFound) {
			return 0, "", err
		} else {
			lastErr = ErrInterfaceNotFound
			if verbose {
				logging.Info("xp2p: waiting for tun interface", "interface", name, "attempt", attempt)
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
