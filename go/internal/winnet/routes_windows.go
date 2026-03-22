//go:build windows

package winnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Route struct {
	DestinationPrefix string `json:"DestinationPrefix"`
	NextHop           string `json:"NextHop"`
	InterfaceIndex    int    `json:"InterfaceIndex"`
	RouteMetric       int    `json:"RouteMetric"`
	PolicyStore       string `json:"PolicyStore"`
	AddressFamily     string `json:"AddressFamily"`
}

var ErrInterfaceNotFound = errors.New("xp2p: interface not found")

func DefaultRoutes(ctx context.Context) ([]Route, error) {
	routes, err := defaultRoutesFromIPHelper()
	if err == nil {
		return routes, nil
	}
	fallback, fallbackErr := defaultRoutesFromPowerShell(ctx)
	if fallbackErr != nil {
		return nil, fmt.Errorf("xp2p: route lookup failed: %v: %w", err, fallbackErr)
	}
	return fallback, nil
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
		routes = append(routes, Route{
			DestinationPrefix: prefix,
			NextHop:           nextHop,
			InterfaceIndex:    int(row.InterfaceIndex),
			RouteMetric:       int(row.Metric),
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

func defaultRoutesFromPowerShell(ctx context.Context) ([]Route, error) {
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$routes = @()`,
		`$routes += Get-NetRoute -DestinationPrefix "0.0.0.0/0" -ErrorAction SilentlyContinue | Select-Object DestinationPrefix,NextHop,InterfaceIndex,RouteMetric,PolicyStore,AddressFamily`,
		`$routes += Get-NetRoute -DestinationPrefix "::/0" -ErrorAction SilentlyContinue | Select-Object DestinationPrefix,NextHop,InterfaceIndex,RouteMetric,PolicyStore,AddressFamily`,
		`if ($routes.Count -eq 0) { Write-Output "" } else { $routes | ConvertTo-Json -Compress }`,
	}, "; ")
	out, err := runPowerShell(ctx, script)
	if err != nil {
		return nil, err
	}
	return decodeRoutes(out)
}

func InterfaceIndexByName(ctx context.Context, name string) (int, error) {
	if idx, err := interfaceIndexByNameNative(name); err == nil {
		return idx, nil
	} else if !errors.Is(err, ErrInterfaceNotFound) {
		return 0, err
	}
	escaped := escapePowerShellString(name)
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$adapter = Get-NetAdapter -Name '` + escaped + `' -ErrorAction SilentlyContinue | Select-Object -First 1`,
		`if ($null -eq $adapter) { Write-Output "" } else { Write-Output $adapter.ifIndex }`,
	}, "; ")
	out, err := runPowerShell(ctx, script)
	if err != nil {
		return 0, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, fmt.Errorf("%w: %s", ErrInterfaceNotFound, name)
	}
	value, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("xp2p: parse interface index: %w", err)
	}
	return value, nil
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

func ApplyRoute(ctx context.Context, route Route) error {
	dest := strings.TrimSpace(route.DestinationPrefix)
	if dest == "" {
		return nil
	}
	nextHop := strings.TrimSpace(route.NextHop)
	if nextHop == "" {
		return fmt.Errorf("xp2p: next hop required for %s", dest)
	}
	if err := applyRouteNative(route); err == nil {
		return nil
	}
	policy := strings.TrimSpace(route.PolicyStore)
	if policy == "" {
		policy = "ActiveStore"
	}
	metric := ""
	if route.RouteMetric > 0 {
		metric = fmt.Sprintf(" -RouteMetric %d", route.RouteMetric)
	}
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$dest = "` + escapePowerShellString(dest) + `"`,
		`$nextHop = "` + escapePowerShellString(nextHop) + `"`,
		`$ifIndex = ` + strconv.Itoa(route.InterfaceIndex),
		`$existing = Get-NetRoute -DestinationPrefix $dest -InterfaceIndex $ifIndex -ErrorAction SilentlyContinue | Select-Object -First 1`,
		`if ($null -eq $existing) {`,
		`  New-NetRoute -DestinationPrefix $dest -InterfaceIndex $ifIndex -NextHop $nextHop -PolicyStore "` + escapePowerShellString(policy) + `"` + metric + ` -ErrorAction Stop | Out-Null`,
		`} else {`,
		`  Set-NetRoute -DestinationPrefix $dest -InterfaceIndex $ifIndex -NextHop $nextHop -PolicyStore "` + escapePowerShellString(policy) + `"` + metric + ` -ErrorAction Stop | Out-Null`,
		`}`,
	}, "; ")
	_, err := runPowerShell(ctx, script)
	return wrapRouteError(err)
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
	if err := removeRouteNative(route); err == nil {
		return nil
	}
	policy := strings.TrimSpace(route.PolicyStore)
	policyArg := ""
	if policy != "" {
		policyArg = ` -PolicyStore "` + escapePowerShellString(policy) + `"`
	}
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$dest = "` + escapePowerShellString(dest) + `"`,
		`$ifIndex = ` + strconv.Itoa(route.InterfaceIndex),
		`$nextHop = "` + escapePowerShellString(nextHop) + `"`,
		`Remove-NetRoute -DestinationPrefix $dest -InterfaceIndex $ifIndex -NextHop $nextHop` + policyArg + ` -Confirm:$false -ErrorAction SilentlyContinue | Out-Null`,
	}, "; ")
	_, err := runPowerShell(ctx, script)
	return wrapRouteError(err)
}

func interfaceIndexByNameNative(name string) (int, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return 0, ErrInterfaceNotFound
	}
	adapter := windows.IpAdapterAddresses{}
	size := uint32(unsafe.Sizeof(adapter))
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, 0, 0, &adapter, &size); err != nil && err != windows.ERROR_BUFFER_OVERFLOW {
		return 0, err
	}
	buf := make([]byte, size)
	head := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, 0, 0, head, &size); err != nil {
		return 0, err
	}
	for aa := head; aa != nil; aa = aa.Next {
		if matchAdapterName(aa, normalized) {
			return int(aa.IfIndex), nil
		}
	}
	return 0, fmt.Errorf("%w: %s", ErrInterfaceNotFound, name)
}

func interfaceIndexByIPNative(addr string) (int, error) {
	ip := parseIP(addr)
	if ip == nil {
		return 0, ErrInterfaceNotFound
	}
	needle := ip.String()
	if needle == "" {
		return 0, ErrInterfaceNotFound
	}
	adapter := windows.IpAdapterAddresses{}
	size := uint32(unsafe.Sizeof(adapter))
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, 0, 0, &adapter, &size); err != nil && err != windows.ERROR_BUFFER_OVERFLOW {
		return 0, err
	}
	buf := make([]byte, size)
	head := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, 0, 0, head, &size); err != nil {
		return 0, err
	}
	for aa := head; aa != nil; aa = aa.Next {
		for ua := aa.FirstUnicastAddress; ua != nil; ua = ua.Next {
			addrIP := ua.Address.IP()
			if addrIP == nil {
				continue
			}
			if addrIP.String() == needle {
				return int(aa.IfIndex), nil
			}
		}
	}
	return 0, ErrInterfaceNotFound
}

func matchAdapterName(adapter *windows.IpAdapterAddresses, target string) bool {
	if adapter == nil || target == "" {
		return false
	}
	if adapter.FriendlyName != nil {
		friendly := strings.TrimSpace(windows.UTF16PtrToString(adapter.FriendlyName))
		if friendly != "" && strings.EqualFold(friendly, target) {
			return true
		}
	}
	if adapter.AdapterName != nil {
		name := strings.TrimSpace(windows.BytePtrToString(adapter.AdapterName))
		if name != "" && strings.EqualFold(name, target) {
			return true
		}
	}
	return false
}

func parseIP(value string) net.IP {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if _, ipNet, err := net.ParseCIDR(value); err == nil && ipNet != nil && ipNet.IP != nil {
		return ipNet.IP
	}
	return net.ParseIP(value)
}

var (
	modiphlpapi               = windows.NewLazySystemDLL("iphlpapi.dll")
	procCreateIpForwardEntry2 = modiphlpapi.NewProc("CreateIpForwardEntry2")
	procSetIpForwardEntry2    = modiphlpapi.NewProc("SetIpForwardEntry2")
	procDeleteIpForwardEntry2 = modiphlpapi.NewProc("DeleteIpForwardEntry2")
)

func applyRouteNative(route Route) error {
	row, err := mibRouteFromRoute(route)
	if err != nil {
		return err
	}
	if err := createIpForwardEntry2(&row); err != nil {
		if errnoIs(err, windows.ERROR_OBJECT_ALREADY_EXISTS) {
			return setIpForwardEntry2(&row)
		}
		return err
	}
	return nil
}

func removeRouteNative(route Route) error {
	row, err := mibRouteFromRoute(route)
	if err != nil {
		return err
	}
	if err := deleteIpForwardEntry2(&row); err != nil {
		if errnoIs(err, windows.ERROR_NOT_FOUND) || errnoIs(err, windows.ERROR_FILE_NOT_FOUND) {
			return nil
		}
		return err
	}
	return nil
}

func mibRouteFromRoute(route Route) (windows.MibIpForwardRow2, error) {
	dest := strings.TrimSpace(route.DestinationPrefix)
	nextHop := strings.TrimSpace(route.NextHop)
	if dest == "" || nextHop == "" {
		return windows.MibIpForwardRow2{}, errors.New("xp2p: route destination and next hop required")
	}
	ip, ipNet, err := net.ParseCIDR(dest)
	if err != nil || ipNet == nil || ip == nil {
		return windows.MibIpForwardRow2{}, fmt.Errorf("xp2p: parse route destination: %s", dest)
	}
	ones, _ := ipNet.Mask.Size()
	prefix, _, ok := rawSockaddrFromIP(ip)
	if !ok {
		return windows.MibIpForwardRow2{}, fmt.Errorf("xp2p: parse route destination: %s", dest)
	}
	nextHopIP := net.ParseIP(nextHop)
	if nextHopIP == nil {
		return windows.MibIpForwardRow2{}, fmt.Errorf("xp2p: parse route next hop: %s", nextHop)
	}
	nextHopRaw, _, ok := rawSockaddrFromIP(nextHopIP)
	if !ok {
		return windows.MibIpForwardRow2{}, fmt.Errorf("xp2p: parse route next hop: %s", nextHop)
	}
	row := windows.MibIpForwardRow2{
		InterfaceIndex:    uint32(route.InterfaceIndex),
		DestinationPrefix: windows.IpAddressPrefix{Prefix: prefix, PrefixLength: uint8(ones)},
		NextHop:           nextHopRaw,
		Metric:            uint32(max(0, route.RouteMetric)),
		Protocol:          windows.MIB_IPPROTO_NETMGMT,
		Origin:            windows.NlroManual,
	}
	return row, nil
}

func rawSockaddrFromIP(ip net.IP) (windows.RawSockaddrInet, string, bool) {
	if ip4 := ip.To4(); ip4 != nil {
		var addr windows.RawSockaddrInet
		raw := (*windows.RawSockaddrInet4)(unsafe.Pointer(&addr))
		raw.Family = windows.AF_INET
		copy(raw.Addr[:], ip4)
		return addr, "IPv4", true
	}
	if ip16 := ip.To16(); ip16 != nil {
		var addr windows.RawSockaddrInet
		raw := (*windows.RawSockaddrInet6)(unsafe.Pointer(&addr))
		raw.Family = windows.AF_INET6
		copy(raw.Addr[:], ip16)
		return addr, "IPv6", true
	}
	return windows.RawSockaddrInet{}, "", false
}

func createIpForwardEntry2(row *windows.MibIpForwardRow2) error {
	if err := procCreateIpForwardEntry2.Find(); err != nil {
		return err
	}
	r0, _, _ := syscall.SyscallN(procCreateIpForwardEntry2.Addr(), uintptr(unsafe.Pointer(row)))
	if r0 != 0 {
		return windows.Errno(r0)
	}
	return nil
}

func setIpForwardEntry2(row *windows.MibIpForwardRow2) error {
	if err := procSetIpForwardEntry2.Find(); err != nil {
		return err
	}
	r0, _, _ := syscall.SyscallN(procSetIpForwardEntry2.Addr(), uintptr(unsafe.Pointer(row)))
	if r0 != 0 {
		return windows.Errno(r0)
	}
	return nil
}

func deleteIpForwardEntry2(row *windows.MibIpForwardRow2) error {
	if err := procDeleteIpForwardEntry2.Find(); err != nil {
		return err
	}
	r0, _, _ := syscall.SyscallN(procDeleteIpForwardEntry2.Addr(), uintptr(unsafe.Pointer(row)))
	if r0 != 0 {
		return windows.Errno(r0)
	}
	return nil
}

func errnoIs(err error, target windows.Errno) bool {
	var errno windows.Errno
	if errors.As(err, &errno) {
		return errno == target
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func decodeRoutes(raw string) ([]Route, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var list []Route
	if err := json.Unmarshal([]byte(trimmed), &list); err == nil {
		return list, nil
	}
	var single Route
	if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
		return nil, fmt.Errorf("xp2p: parse routes: %w", err)
	}
	return []Route{single}, nil
}

func wrapRouteError(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "access is denied") ||
		strings.Contains(lower, "access denied") ||
		strings.Contains(lower, "requested operation requires elevation") {
		return fmt.Errorf("xp2p: route change requires administrator privileges: %w", err)
	}
	return err
}
