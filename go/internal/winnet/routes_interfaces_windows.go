//go:build windows

package winnet

import (
	"fmt"
	"net"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

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
