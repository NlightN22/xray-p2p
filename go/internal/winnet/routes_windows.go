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
	"syscall"
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

var ErrInterfaceNotFound = errors.New("xp2p: interface not found")

func DefaultRoutes(ctx context.Context) ([]Route, error) {
	routes, err := defaultRoutesFromIPHelper()
	if err == nil {
		return routes, nil
	}
	return nil, fmt.Errorf("xp2p: route lookup failed: %w", err)
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
		return fmt.Errorf("xp2p: next hop required for %s", dest)
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

func interfaceLuidByNameNative(name string) (uint64, error) {
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
			return aa.Luid, nil
		}
	}
	return 0, fmt.Errorf("%w: %s", ErrInterfaceNotFound, name)
}

func interfaceLuidByIPNative(addr string) (uint64, error) {
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
				return aa.Luid, nil
			}
		}
	}
	return 0, ErrInterfaceNotFound
}

func interfaceIPv4Native(ifIndex int) (string, error) {
	adapter := windows.IpAdapterAddresses{}
	size := uint32(unsafe.Sizeof(adapter))
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, 0, 0, &adapter, &size); err != nil && err != windows.ERROR_BUFFER_OVERFLOW {
		return "", err
	}
	buf := make([]byte, size)
	head := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, 0, 0, head, &size); err != nil {
		return "", err
	}
	best := ""
	bestPrefix := uint8(0)
	for aa := head; aa != nil; aa = aa.Next {
		if int(aa.IfIndex) != ifIndex {
			continue
		}
		for ua := aa.FirstUnicastAddress; ua != nil; ua = ua.Next {
			addrIP := ua.Address.IP()
			if addrIP == nil {
				continue
			}
			ip4 := addrIP.To4()
			if ip4 == nil {
				continue
			}
			if ip4.Equal(net.IPv4zero) || ip4.IsLoopback() || (ip4[0] == 169 && ip4[1] == 254) {
				continue
			}
			prefix := ua.OnLinkPrefixLength
			if best == "" || prefix > bestPrefix {
				best = ip4.String()
				bestPrefix = prefix
			}
		}
	}
	return best, nil
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
		return nil, errors.New("xp2p: route destination required")
	}
	nextHop := strings.TrimSpace(route.NextHop)
	if nextHop == "" {
		nextHop = "0.0.0.0"
	}
	ifIndex := route.InterfaceIndex
	if ifIndex <= 0 {
		return nil, errors.New("xp2p: interface index required")
	}
	metric := route.RouteMetric
	ip, ipNet, err := net.ParseCIDR(dest)
	if err != nil || ipNet == nil || ip == nil {
		return nil, fmt.Errorf("xp2p: parse route destination: %s", dest)
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
	return fmt.Errorf("xp2p: route.exe failed: %w: %s", err, output)
}

var (
	modiphlpapi                    = windows.NewLazySystemDLL("iphlpapi.dll")
	procCreateIpForwardEntry2      = modiphlpapi.NewProc("CreateIpForwardEntry2")
	procSetIpForwardEntry2         = modiphlpapi.NewProc("SetIpForwardEntry2")
	procDeleteIpForwardEntry2      = modiphlpapi.NewProc("DeleteIpForwardEntry2")
	procInitializeIpForwardEntry2  = modiphlpapi.NewProc("InitializeIpForwardEntry2")
	procInitializeIpInterfaceEntry = modiphlpapi.NewProc("InitializeIpInterfaceEntry")
	procGetIpInterfaceEntry        = modiphlpapi.NewProc("GetIpInterfaceEntry")
)

func IsRouteNotFoundError(err error) bool {
	return isRouteNotFoundError(err)
}

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
	var row windows.MibIpForwardRow2
	if err := initializeIpForwardEntry2(&row); err != nil {
		return windows.MibIpForwardRow2{}, err
	}
	if route.InterfaceLuid != 0 {
		row.InterfaceLuid = route.InterfaceLuid
	} else {
		row.InterfaceIndex = uint32(route.InterfaceIndex)
	}
	row.DestinationPrefix = windows.IpAddressPrefix{Prefix: prefix, PrefixLength: uint8(ones)}
	row.NextHop = nextHopRaw
	row.Metric = uint32(max(0, route.RouteMetric))
	row.Protocol = windows.MIB_IPPROTO_NETMGMT
	row.Origin = windows.NlroManual
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

func initializeIpForwardEntry2(row *windows.MibIpForwardRow2) error {
	if err := procInitializeIpForwardEntry2.Find(); err != nil {
		return err
	}
	r0, _, _ := syscall.SyscallN(procInitializeIpForwardEntry2.Addr(), uintptr(unsafe.Pointer(row)))
	if r0 != 0 {
		return windows.Errno(r0)
	}
	return nil
}

func initializeIpInterfaceEntry(row *windows.MibIpInterfaceRow) error {
	if err := procInitializeIpInterfaceEntry.Find(); err != nil {
		return err
	}
	r0, _, _ := syscall.SyscallN(procInitializeIpInterfaceEntry.Addr(), uintptr(unsafe.Pointer(row)))
	if r0 != 0 {
		return windows.Errno(r0)
	}
	return nil
}

func getIpInterfaceEntry(row *windows.MibIpInterfaceRow) error {
	if err := procGetIpInterfaceEntry.Find(); err != nil {
		return err
	}
	r0, _, _ := syscall.SyscallN(procGetIpInterfaceEntry.Addr(), uintptr(unsafe.Pointer(row)))
	if r0 != 0 {
		return windows.Errno(r0)
	}
	return nil
}

func interfaceMetricFromIPHelper(luid uint64, ifIndex int, family string) (int, bool) {
	var row windows.MibIpInterfaceRow
	if err := initializeIpInterfaceEntry(&row); err != nil {
		return 0, false
	}
	if strings.EqualFold(family, "IPv6") {
		row.Family = windows.AF_INET6
	} else {
		row.Family = windows.AF_INET
	}
	if luid != 0 {
		row.InterfaceLuid = luid
	} else if ifIndex > 0 {
		row.InterfaceIndex = uint32(ifIndex)
	} else {
		return 0, false
	}
	if err := getIpInterfaceEntry(&row); err != nil {
		return 0, false
	}
	return int(row.Metric), true
}

func errnoIs(err error, target windows.Errno) bool {
	var errno windows.Errno
	if errors.As(err, &errno) {
		return errno == target
	}
	return false
}

func isRouteNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errnoIs(err, windows.ERROR_NOT_FOUND) || errnoIs(err, windows.ERROR_FILE_NOT_FOUND) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "element not found") || strings.Contains(lower, "error 1168")
}

func isIPHelperUnsupported(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "initializeipforwardentry2") ||
		strings.Contains(lower, "procedure could not be found")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
