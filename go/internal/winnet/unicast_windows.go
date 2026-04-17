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
	modiphlpapiUnicast                  = windows.NewLazySystemDLL("iphlpapi.dll")
	procInitializeUnicastIpAddressEntry = modiphlpapiUnicast.NewProc("InitializeUnicastIpAddressEntry")
	procCreateUnicastIpAddressEntry     = modiphlpapiUnicast.NewProc("CreateUnicastIpAddressEntry")
	procSetUnicastIpAddressEntry        = modiphlpapiUnicast.NewProc("SetUnicastIpAddressEntry")
)

const ipLifetimeInfinite uint32 = 0xFFFFFFFF

func assignInterfaceIPv4Native(ifIndex int, ip string, prefix int) error {
	addr := strings.TrimSpace(ip)
	if ifIndex <= 0 || addr == "" || prefix <= 0 {
		return ErrInterfaceNotFound
	}
	row, err := unicastRowFromIPv4(ifIndex, addr, prefix)
	if err != nil {
		return err
	}
	if err := createUnicastIpAddressEntry(&row); err != nil {
		if isUnicastEntryExistsError(err) {
			return setUnicastIpAddressEntry(&row)
		}
		return err
	}
	return nil
}

func unicastRowFromIPv4(ifIndex int, addr string, prefix int) (windows.MibUnicastIpAddressRow, error) {
	ip := net.ParseIP(strings.TrimSpace(addr))
	if ip == nil {
		return windows.MibUnicastIpAddressRow{}, fmt.Errorf("parse ip address: %s", addr)
	}
	raw, family, ok := rawSockaddrFromIPUnicast(ip)
	if !ok || family != "IPv4" {
		return windows.MibUnicastIpAddressRow{}, fmt.Errorf("ipv4 address required: %s", addr)
	}
	var row windows.MibUnicastIpAddressRow
	if err := initializeUnicastIpAddressEntry(&row); err != nil {
		return windows.MibUnicastIpAddressRow{}, err
	}
	row.InterfaceIndex = uint32(ifIndex)
	row.Address = raw
	row.OnLinkPrefixLength = uint8(prefix)
	row.PrefixOrigin = windows.IpPrefixOriginManual
	row.SuffixOrigin = windows.IpSuffixOriginManual
	row.ValidLifetime = ipLifetimeInfinite
	row.PreferredLifetime = ipLifetimeInfinite
	return row, nil
}

func rawSockaddrFromIPUnicast(ip net.IP) (windows.RawSockaddrInet6, string, bool) {
	if ip4 := ip.To4(); ip4 != nil {
		var addr windows.RawSockaddrInet6
		addr.Family = windows.AF_INET
		copy(addr.Addr[:], ip4)
		return addr, "IPv4", true
	}
	if ip16 := ip.To16(); ip16 != nil {
		var addr windows.RawSockaddrInet6
		addr.Family = windows.AF_INET6
		copy(addr.Addr[:], ip16)
		return addr, "IPv6", true
	}
	return windows.RawSockaddrInet6{}, "", false
}

func initializeUnicastIpAddressEntry(row *windows.MibUnicastIpAddressRow) error {
	if err := procInitializeUnicastIpAddressEntry.Find(); err != nil {
		return err
	}
	r0, _, _ := syscall.SyscallN(procInitializeUnicastIpAddressEntry.Addr(), uintptr(unsafe.Pointer(row)))
	if r0 != 0 {
		return windows.Errno(r0)
	}
	return nil
}

func createUnicastIpAddressEntry(row *windows.MibUnicastIpAddressRow) error {
	if err := procCreateUnicastIpAddressEntry.Find(); err != nil {
		return err
	}
	r0, _, _ := syscall.SyscallN(procCreateUnicastIpAddressEntry.Addr(), uintptr(unsafe.Pointer(row)))
	if r0 != 0 {
		return windows.Errno(r0)
	}
	return nil
}

func setUnicastIpAddressEntry(row *windows.MibUnicastIpAddressRow) error {
	if err := procSetUnicastIpAddressEntry.Find(); err != nil {
		return err
	}
	r0, _, _ := syscall.SyscallN(procSetUnicastIpAddressEntry.Addr(), uintptr(unsafe.Pointer(row)))
	if r0 != 0 {
		return windows.Errno(r0)
	}
	return nil
}

func isUnicastIPHelperUnsupported(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ERROR_PROC_NOT_FOUND) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "initializeunicastipaddressentry") ||
		strings.Contains(lower, "createunicastipaddressentry") ||
		strings.Contains(lower, "setunicastipaddressentry") ||
		strings.Contains(lower, "procedure could not be found")
}

func isUnicastEntryExistsError(err error) bool {
	if err == nil {
		return false
	}
	if errnoIs(err, windows.ERROR_OBJECT_ALREADY_EXISTS) || errnoIs(err, windows.ERROR_DIR_NOT_EMPTY) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "directory cannot be removed") || strings.Contains(lower, "already exists")
}
