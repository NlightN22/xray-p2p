//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/ports"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func (m *TunnelManager) Status(ctx context.Context, req ports.TunStatusRequest) (ports.TunStatus, error) {
	if err := ctx.Err(); err != nil {
		return ports.TunStatus{}, err
	}
	ifIndex, err := resolveTunIndex(ctx, req.Name, req.Addr)
	if err != nil {
		return ports.TunStatus{}, err
	}
	details, err := winnet.InterfaceIPv4Details(ifIndex)
	status := ports.TunStatus{
		IfIndex:    ifIndex,
		IP:         strings.TrimSpace(details.IP),
		Prefix:     int(details.Prefix),
		OperStatus: winnet.InterfaceOperStatusName(details.OperStatus),
		DadState:   winnet.InterfaceDadStateName(details.DadState),
	}
	if err != nil && !errors.Is(err, winnet.ErrTunIPv4Missing) {
		return status, err
	}

	status.Ready = isReadyStatus(status)
	if req.RequireIPv4 && status.IP == "" {
		return status, winnet.ErrTunIPv4Missing
	}
	if req.RequireUp && !strings.EqualFold(status.OperStatus, "up") {
		return status, fmt.Errorf("tun interface not up (%s)", status.OperStatus)
	}
	if req.RequireReady && !status.Ready {
		return status, fmt.Errorf("tun interface not ready (%s/%s)", status.OperStatus, status.DadState)
	}
	return status, nil
}

func (m *TunnelManager) EnsureReady(ctx context.Context, req ports.TunEnsureRequest) (ports.TunStatus, error) {
	if err := ctx.Err(); err != nil {
		return ports.TunStatus{}, err
	}
	timeout := parseDuration(req.Timeout, 60*time.Second)
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ifIndex, ip, err := winnet.WaitForTunIPv4(waitCtx, req.Name, req.Addr, req.Verbose)
	status := ports.TunStatus{
		IfIndex: ifIndex,
		IP:      strings.TrimSpace(ip),
	}
	if err != nil {
		return status, err
	}
	if details, detailErr := winnet.InterfaceIPv4Details(ifIndex); detailErr == nil {
		status.Prefix = int(details.Prefix)
		status.OperStatus = winnet.InterfaceOperStatusName(details.OperStatus)
		status.DadState = winnet.InterfaceDadStateName(details.DadState)
		status.Ready = isReadyStatus(status)
	}
	return status, nil
}

func resolveTunIndex(ctx context.Context, name, addr string) (int, error) {
	if strings.TrimSpace(addr) != "" {
		ifIndex, err := winnet.InterfaceIndexByIP(addr)
		if err == nil && ifIndex > 0 {
			return ifIndex, nil
		}
		if err != nil && !errors.Is(err, winnet.ErrInterfaceNotFound) {
			return 0, err
		}
	}
	if strings.TrimSpace(name) == "" {
		return 0, winnet.ErrInterfaceNotFound
	}
	ifIndex, _, _, err := winnet.InterfaceByNamePrefix(name)
	if err == nil && ifIndex > 0 {
		return ifIndex, nil
	}
	ifIndex, err = winnet.InterfaceIndexByName(ctx, name)
	if err == nil && ifIndex > 0 {
		return ifIndex, nil
	}
	if errors.Is(err, winnet.ErrInterfaceNotFound) {
		ifIndex, _, _, err = winnet.InterfaceByDescriptionContains([]string{"xray tunnel", "wintun"})
		if err == nil && ifIndex > 0 {
			return ifIndex, nil
		}
	}
	if err != nil {
		return 0, err
	}
	return 0, winnet.ErrInterfaceNotFound
}

func isReadyStatus(status ports.TunStatus) bool {
	return status.IP != "" &&
		strings.EqualFold(status.OperStatus, "up") &&
		strings.EqualFold(status.DadState, "preferred")
}
