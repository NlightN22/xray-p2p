//go:build linux

package client

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func listDefaultRoutes(family string) ([]string, error) {
	if _, err := exec.LookPath("ip"); err != nil {
		return nil, errors.New("ip command not found")
	}
	out, err := captureIPCommand(family, "route", "show", "default")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	var routes []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		routes = append(routes, trimmed)
	}
	return routes, nil
}

func removeDefaultRoutes(routes []string, family string) error {
	for _, route := range routes {
		if err := runIPCommand(family, append([]string{"route", "del"}, strings.Fields(route)...)...); err != nil {
			if isMissingRouteError(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func restoreDefaultRoutes(routes []string, family string) error {
	for _, route := range routes {
		if err := runIPCommand(family, append([]string{"route", "replace"}, strings.Fields(route)...)...); err != nil {
			return err
		}
	}
	return nil
}

func ensureDefaultRoute(tunName string, family string) error {
	name := strings.TrimSpace(tunName)
	if name == "" {
		return errors.New("tun name is required for full-tunnel default route")
	}
	return runIPCommand(family, "route", "replace", "default", "dev", name)
}

func removeTunDefaultRoute(tunName string, family string) error {
	name := strings.TrimSpace(tunName)
	if name == "" {
		return nil
	}
	if err := runIPCommand(family, "route", "del", "default", "dev", name); err != nil {
		if isMissingRouteError(err) {
			return nil
		}
		return err
	}
	return nil
}

func buildBypassRoutes(defaults []string, ips []string, prefixLen int) []string {
	if len(defaults) == 0 || len(ips) == 0 {
		return nil
	}
	var routes []string
	for _, ip := range ips {
		dest := fmt.Sprintf("%s/%d", ip, prefixLen)
		for _, def := range defaults {
			fields := strings.Fields(def)
			if len(fields) == 0 {
				continue
			}
			if fields[0] != "default" {
				continue
			}
			fields[0] = dest
			route := strings.Join(fields, " ")
			routes = append(routes, route)
		}
	}
	return routes
}

func syncBypassRoutes(desired []string, existing []fullTunnelRoute, family string) error {
	desiredSet := make(map[string]struct{}, len(desired))
	for _, route := range desired {
		desiredSet[route] = struct{}{}
	}
	for _, route := range existing {
		if strings.ToLower(route.Family) == familyKey(family) && route.Route != "" {
			if _, ok := desiredSet[route.Route]; !ok {
				if err := removeRoute(route.Route, family); err != nil {
					return err
				}
			}
		}
	}
	for _, route := range desired {
		if err := runIPCommand(family, append([]string{"route", "replace"}, strings.Fields(route)...)...); err != nil {
			return err
		}
	}
	return nil
}

func removeStoredBypassRoutes(routes []fullTunnelRoute) error {
	for _, route := range routes {
		family := "-4"
		if strings.EqualFold(route.Family, "ipv6") {
			family = "-6"
		}
		if route.Route == "" {
			continue
		}
		if err := removeRoute(route.Route, family); err != nil {
			return err
		}
	}
	return nil
}

func removeRoute(route string, family string) error {
	if err := runIPCommand(family, append([]string{"route", "del"}, strings.Fields(route)...)...); err != nil {
		if isMissingRouteError(err) {
			return nil
		}
		return err
	}
	return nil
}

func familyKey(family string) string {
	if strings.Contains(family, "6") {
		return "ipv6"
	}
	return "ipv4"
}

func runIPCommand(family string, args ...string) error {
	cmdArgs := append([]string{}, args...)
	if family == "-6" || family == "-4" {
		cmdArgs = append([]string{family}, cmdArgs...)
	}
	cmd := exec.Command("ip", cmdArgs...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ip %s: %v (%s)", strings.Join(cmdArgs, " "), err, strings.TrimSpace(buf.String()))
	}
	return nil
}

func captureIPCommand(args ...string) (string, error) {
	cmd := exec.Command("ip", args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ip %s: %v (%s)", strings.Join(args, " "), err, strings.TrimSpace(buf.String()))
	}
	return strings.TrimSpace(buf.String()), nil
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
