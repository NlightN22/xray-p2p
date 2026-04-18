//go:build windows

package winnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

func applyRouteLegacy(ctx context.Context, route Route) error {
	args, err := buildRouteArgs("add", route)
	if err != nil {
		return err
	}
	return runRouteCommand(ctx, args, false)
}

func removeRouteLegacy(ctx context.Context, route Route) error {
	args, err := buildRouteArgs("delete", route)
	if err != nil {
		return err
	}
	return runRouteCommand(ctx, args, true)
}

func buildRouteArgs(action string, route Route) ([]string, error) {
	dest := strings.TrimSpace(route.DestinationPrefix)
	if dest == "" {
		return nil, errors.New("route destination required")
	}
	nextHop := strings.TrimSpace(route.NextHop)
	if nextHop == "" {
		nextHop = "0.0.0.0"
	}
	ifIndex := route.InterfaceIndex
	if ifIndex <= 0 {
		return nil, errors.New("interface index required")
	}
	metric := route.RouteMetric
	ip, ipNet, err := net.ParseCIDR(dest)
	if err != nil || ipNet == nil || ip == nil {
		return nil, fmt.Errorf("parse route destination: %s", dest)
	}
	if ip.To4() == nil {
		args := []string{"-6", action, dest, nextHop, "if", strconv.Itoa(ifIndex)}
		if metric > 0 {
			args = append(args, "metric", strconv.Itoa(metric))
		}
		return args, nil
	}
	mask := net.IP(ipNet.Mask).String()
	args := []string{action, ip.String(), "mask", mask, nextHop, "if", strconv.Itoa(ifIndex)}
	if metric > 0 {
		args = append(args, "metric", strconv.Itoa(metric))
	}
	return args, nil
}

func runRouteCommand(ctx context.Context, args []string, ignoreNotFound bool) error {
	routePath, err := lookPathSystem32("route.exe")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, routePath, args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	output := strings.TrimSpace(string(out))
	if ignoreNotFound {
		lower := strings.ToLower(output)
		if strings.Contains(lower, "not found") || strings.Contains(lower, "no such") {
			return nil
		}
	}
	return fmt.Errorf("route.exe failed: %w: %s", err, output)
}
