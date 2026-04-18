//go:build windows

package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func resolveWindowsInterface(ctx context.Context, tunName string, tunAddr string, verbose bool, wait bool) (int, uint64, error) {
	name := strings.TrimSpace(tunName)
	if name == "" {
		return 0, 0, errors.New("tun name is required for interface lookup")
	}
	trimmedAddr := strings.TrimSpace(tunAddr)
	deadline := time.Now()
	if wait {
		deadline = time.Now().Add(10 * time.Second)
	}
	attempt := 0
	var lastErr error
	for {
		attempt++
		if trimmedAddr != "" {
			ifIndex, addrErr := winnet.InterfaceIndexByIP(trimmedAddr)
			if addrErr == nil && ifIndex > 0 {
				luid, luidErr := winnet.InterfaceLuidByIP(trimmedAddr)
				if luidErr != nil {
					luid = 0
				}
				if verbose {
					logging.Info("full-tunnel tun interface resolved by addr", "interface", name, "addr", trimmedAddr, "ifIndex", ifIndex, "ifLuid", luid, "attempt", attempt)
				}
				return ifIndex, luid, nil
			}
			if addrErr != nil && !errors.Is(addrErr, winnet.ErrInterfaceNotFound) {
				lastErr = addrErr
			}
		}
		ifIndex, ifLuid, matched, matchErr := winnet.InterfaceByNamePrefix(name)
		if matchErr == nil && ifIndex > 0 {
			if verbose {
				logging.Info("full-tunnel tun interface resolved by prefix", "interface", name, "match", matched, "ifIndex", ifIndex, "ifLuid", ifLuid, "attempt", attempt)
			}
			return ifIndex, ifLuid, nil
		}
		index, nameErr := winnet.InterfaceIndexByName(ctx, name)
		if nameErr == nil {
			luid, luidErr := winnet.InterfaceLuidByName(name)
			if luidErr != nil {
				luid = 0
			}
			if verbose {
				logging.Info("full-tunnel tun interface resolved", "interface", name, "ifIndex", index, "ifLuid", luid, "attempt", attempt)
			}
			return index, luid, nil
		}
		lastErr = nameErr
		if errors.Is(lastErr, winnet.ErrInterfaceNotFound) {
			hints := []string{"xray tunnel", "wintun"}
			ifIndex, ifLuid, matched, matchErr = winnet.InterfaceByDescriptionContains(hints)
			if matchErr == nil && ifIndex > 0 {
				if verbose {
					logging.Info("full-tunnel tun interface resolved by description", "interface", name, "match", matched, "ifIndex", ifIndex, "ifLuid", ifLuid, "attempt", attempt)
				}
				return ifIndex, ifLuid, nil
			}
		}
		if ctx.Err() != nil {
			return 0, 0, lastErr
		}
		if lastErr != nil && !errors.Is(lastErr, winnet.ErrInterfaceNotFound) {
			return 0, 0, lastErr
		}
		if time.Now().After(deadline) {
			return 0, 0, lastErr
		}
		if verbose {
			logging.Info("full-tunnel waiting for tun interface", "interface", name, "attempt", attempt)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitForWindowsIPv4(ctx context.Context, ifIndex int, verbose bool) error {
	deadline := time.Now().Add(10 * time.Second)
	attempt := 0
	for {
		attempt++
		value, err := winnet.InterfaceIPv4(ctx, ifIndex)
		if err != nil {
			return err
		}
		if strings.TrimSpace(value) != "" {
			if verbose {
				logging.Info("full-tunnel tun IPv4 ready", "ifIndex", ifIndex, "ip", value, "attempt", attempt)
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("tun IPv4 address unavailable for interface %d", ifIndex)
		}
		if verbose {
			logging.Info("full-tunnel waiting for tun IPv4", "ifIndex", ifIndex, "attempt", attempt)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitForWindowsInterfaceUp(ctx context.Context, ifIndex int, tunName string, verbose bool) error {
	deadline := time.Now().Add(20 * time.Second)
	attempt := 0
	logged := false
	for {
		attempt++
		up, err := winnet.InterfaceIsUpByIndex(ifIndex)
		if err != nil {
			return err
		}
		if up {
			return nil
		}
		if !logged {
			logging.Info("full-tunnel apply deferred: adapter not connected", "interface", tunName, "ifIndex", ifIndex)
			logged = true
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("tun adapter not connected: %s (%d)", tunName, ifIndex)
		}
		if verbose {
			logging.Info("full-tunnel waiting for tun adapter", "interface", tunName, "ifIndex", ifIndex, "attempt", attempt)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
