//go:build windows

package winnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Route struct {
	DestinationPrefix string `json:"DestinationPrefix"`
	NextHop           string `json:"NextHop"`
	InterfaceIndex    int    `json:"InterfaceIndex"`
	InterfaceLuid     uint64 `json:"InterfaceLuid"`
	RouteMetric       int    `json:"RouteMetric"`
	InterfaceMetric   int    `json:"InterfaceMetric"`
	PolicyStore       string `json:"PolicyStore"`
	AddressFamily     string `json:"AddressFamily"`
}

var ErrInterfaceNotFound = errors.New("interface not found")

func DefaultRoutes(ctx context.Context) ([]Route, error) {
	routes, err := defaultRoutesFromIPHelper()
	if err == nil {
		return routes, nil
	}
	return nil, fmt.Errorf("route lookup failed: %w", err)
}

// BestRouteForIP returns the most specific route that matches the given IP.
func BestRouteForIP(ip string) (Route, int, bool, error) {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return Route{}, 0, false, nil
	}
	family := "IPv6"
	if parsed.To4() != nil {
		family = "IPv4"
	}
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(windows.AF_UNSPEC, &table); err != nil {
		return Route{}, 0, false, err
	}
	if table == nil {
		return Route{}, 0, false, nil
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))
	var best Route
	bestPrefix := -1
	bestMetric := 0
	for _, row := range table.Rows() {
		prefix, fam, ok := ipPrefixFromRaw(row.DestinationPrefix)
		if !ok || !strings.EqualFold(fam, family) {
			continue
		}
		_, ipNet, err := net.ParseCIDR(prefix)
		if err != nil || ipNet == nil {
			continue
		}
		if !ipNet.Contains(parsed) {
			continue
		}
		prefixLen, _ := ipNet.Mask.Size()
		ifMetric, _ := interfaceMetricFromIPHelper(row.InterfaceLuid, int(row.InterfaceIndex), fam)
		metric := int(row.Metric) + ifMetric
		if prefixLen > bestPrefix || (prefixLen == bestPrefix && (bestPrefix < 0 || metric < bestMetric)) {
			nextHop, _, _ := ipFromRaw(row.NextHop)
			best = Route{
				DestinationPrefix: prefix,
				NextHop:           nextHop,
				InterfaceIndex:    int(row.InterfaceIndex),
				InterfaceLuid:     row.InterfaceLuid,
				RouteMetric:       int(row.Metric),
				InterfaceMetric:   ifMetric,
				PolicyStore:       "ActiveStore",
				AddressFamily:     fam,
			}
			bestPrefix = prefixLen
			bestMetric = metric
		}
	}
	if bestPrefix < 0 {
		return Route{}, 0, false, nil
	}
	return best, bestPrefix, true, nil
}

func defaultRoutesFromIPHelper() ([]Route, error) {
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(windows.AF_UNSPEC, &table); err != nil {
		return nil, err
	}
	if table == nil {
		return nil, nil
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))
	routes := make([]Route, 0, 2)
	for _, row := range table.Rows() {
		prefix, family, ok := ipPrefixFromRaw(row.DestinationPrefix)
		if !ok {
			continue
		}
		if prefix != "0.0.0.0/0" && prefix != "::/0" {
			continue
		}
		nextHop, _, _ := ipFromRaw(row.NextHop)
		ifMetric, _ := interfaceMetricFromIPHelper(row.InterfaceLuid, int(row.InterfaceIndex), family)
		routes = append(routes, Route{
			DestinationPrefix: prefix,
			NextHop:           nextHop,
			InterfaceIndex:    int(row.InterfaceIndex),
			InterfaceLuid:     row.InterfaceLuid,
			RouteMetric:       int(row.Metric),
			InterfaceMetric:   ifMetric,
			PolicyStore:       "ActiveStore",
			AddressFamily:     family,
		})
	}
	return routes, nil
}

func ipPrefixFromRaw(prefix windows.IpAddressPrefix) (string, string, bool) {
	ip, family, ok := ipFromRaw(prefix.Prefix)
	if !ok || ip == "" {
		return "", "", false
	}
	return fmt.Sprintf("%s/%d", ip, prefix.PrefixLength), family, true
}

func ipFromRaw(addr windows.RawSockaddrInet) (string, string, bool) {
	switch addr.Family {
	case windows.AF_INET:
		raw := (*windows.RawSockaddrInet4)(unsafe.Pointer(&addr))
		ip := net.IP(raw.Addr[:]).To4()
		if ip == nil {
			return "", "", false
		}
		return ip.String(), "IPv4", true
	case windows.AF_INET6:
		raw := (*windows.RawSockaddrInet6)(unsafe.Pointer(&addr))
		ip := net.IP(raw.Addr[:])
		if ip == nil {
			return "", "", false
		}
		return ip.String(), "IPv6", true
	default:
		return "", "", false
	}
}

func InterfaceIndexByName(ctx context.Context, name string) (int, error) {
	if idx, err := interfaceIndexByNameNative(name); err == nil {
		return idx, nil
	} else {
		return 0, err
	}
}

func InterfaceIndexByIP(addr string) (int, error) {
	if idx, err := interfaceIndexByIPNative(addr); err == nil {
		return idx, nil
	} else if errors.Is(err, ErrInterfaceNotFound) {
		return 0, err
	} else {
		return 0, err
	}
}

func InterfaceLuidByName(name string) (uint64, error) {
	if luid, err := interfaceLuidByNameNative(name); err == nil {
		return luid, nil
	} else if errors.Is(err, ErrInterfaceNotFound) {
		return 0, err
	} else {
		return 0, err
	}
}

func InterfaceLuidByIP(addr string) (uint64, error) {
	if luid, err := interfaceLuidByIPNative(addr); err == nil {
		return luid, nil
	} else if errors.Is(err, ErrInterfaceNotFound) {
		return 0, err
	} else {
		return 0, err
	}
}

func ApplyRoute(ctx context.Context, route Route) error {
	dest := strings.TrimSpace(route.DestinationPrefix)
	if dest == "" {
		return nil
	}
	nextHop := strings.TrimSpace(route.NextHop)
	if nextHop == "" {
		return fmt.Errorf("next hop required for %s", dest)
	}
	if err := applyRouteNative(route); err != nil {
		if isIPHelperUnsupported(err) {
			return wrapRouteError(applyRouteLegacy(ctx, route))
		}
		return wrapRouteError(err)
	}
	return nil
}

func InterfaceIPv4(ctx context.Context, ifIndex int) (string, error) {
	if ifIndex <= 0 {
		return "", nil
	}
	value, err := interfaceIPv4Native(ifIndex)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func RemoveRoute(ctx context.Context, route Route) error {
	dest := strings.TrimSpace(route.DestinationPrefix)
	if dest == "" {
		return nil
	}
	nextHop := strings.TrimSpace(route.NextHop)
	if nextHop == "" {
		nextHop = "0.0.0.0"
	}
	if err := removeRouteNative(route); err != nil {
		if isIPHelperUnsupported(err) {
			return wrapRouteError(removeRouteLegacy(ctx, route))
		}
		return wrapRouteError(err)
	}
	return nil
}
