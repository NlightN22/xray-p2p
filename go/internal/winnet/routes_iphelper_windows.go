//go:build windows

package winnet

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

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
		return windows.MibIpForwardRow2{}, errors.New("route destination and next hop required")
	}
	ip, ipNet, err := net.ParseCIDR(dest)
	if err != nil || ipNet == nil || ip == nil {
		return windows.MibIpForwardRow2{}, fmt.Errorf("parse route destination: %s", dest)
	}
	ones, _ := ipNet.Mask.Size()
	prefix, _, ok := rawSockaddrFromIP(ip)
	if !ok {
		return windows.MibIpForwardRow2{}, fmt.Errorf("parse route destination: %s", dest)
	}
	nextHopIP := net.ParseIP(nextHop)
	if nextHopIP == nil {
		return windows.MibIpForwardRow2{}, fmt.Errorf("parse route next hop: %s", nextHop)
	}
	nextHopRaw, _, ok := rawSockaddrFromIP(nextHopIP)
	if !ok {
		return windows.MibIpForwardRow2{}, fmt.Errorf("parse route next hop: %s", nextHop)
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
		return fmt.Errorf("route change requires administrator privileges: %w", err)
	}
	return err
}
