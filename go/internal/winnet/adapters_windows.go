//go:build windows

package winnet

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type adapterInfo struct {
	FriendlyName string
	AdapterName  string
	Description  string
	IfIndex      int
	Luid         uint64
	OperStatus   uint32
}

func InterfaceByNamePrefix(prefix string) (int, uint64, string, error) {
	normalized := strings.TrimSpace(prefix)
	if normalized == "" {
		return 0, 0, "", ErrInterfaceNotFound
	}
	adapters, err := adapterInfos()
	if err != nil {
		return 0, 0, "", err
	}
	var candidates []adapterInfo
	for _, adapter := range adapters {
		if hasPrefixFold(adapter.FriendlyName, normalized) || hasPrefixFold(adapter.AdapterName, normalized) {
			candidates = append(candidates, adapter)
		}
	}
	best, ok := pickBestAdapter(candidates)
	if !ok {
		return 0, 0, "", ErrInterfaceNotFound
	}
	return best.IfIndex, best.Luid, adapterDisplayName(best), nil
}

func InterfaceByDescriptionContains(fragments []string) (int, uint64, string, error) {
	cleaned := normalizeFragments(fragments)
	if len(cleaned) == 0 {
		return 0, 0, "", ErrInterfaceNotFound
	}
	adapters, err := adapterInfos()
	if err != nil {
		return 0, 0, "", err
	}
	var candidates []adapterInfo
	for _, adapter := range adapters {
		if containsAnyFold(adapter.Description, cleaned) || containsAnyFold(adapter.FriendlyName, cleaned) {
			candidates = append(candidates, adapter)
		}
	}
	best, ok := pickBestAdapter(candidates)
	if !ok {
		return 0, 0, "", ErrInterfaceNotFound
	}
	return best.IfIndex, best.Luid, adapterDisplayName(best), nil
}

func adapterInfos() ([]adapterInfo, error) {
	adapter := windows.IpAdapterAddresses{}
	size := uint32(unsafe.Sizeof(adapter))
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, 0, 0, &adapter, &size); err != nil && err != windows.ERROR_BUFFER_OVERFLOW {
		return nil, err
	}
	buf := make([]byte, size)
	head := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, 0, 0, head, &size); err != nil {
		return nil, err
	}
	var out []adapterInfo
	for aa := head; aa != nil; aa = aa.Next {
		info := adapterInfo{
			IfIndex:    int(aa.IfIndex),
			Luid:       aa.Luid,
			OperStatus: aa.OperStatus,
		}
		if aa.FriendlyName != nil {
			info.FriendlyName = strings.TrimSpace(windows.UTF16PtrToString(aa.FriendlyName))
		}
		if aa.Description != nil {
			info.Description = strings.TrimSpace(windows.UTF16PtrToString(aa.Description))
		}
		if aa.AdapterName != nil {
			info.AdapterName = strings.TrimSpace(windows.BytePtrToString(aa.AdapterName))
		}
		out = append(out, info)
	}
	return out, nil
}

func InterfaceOperStatusByIndex(ifIndex int) (uint32, error) {
	if ifIndex <= 0 {
		return 0, ErrInterfaceNotFound
	}
	adapters, err := adapterInfos()
	if err != nil {
		return 0, err
	}
	for _, adapter := range adapters {
		if adapter.IfIndex == ifIndex {
			return adapter.OperStatus, nil
		}
	}
	return 0, ErrInterfaceNotFound
}

func InterfaceIsUpByIndex(ifIndex int) (bool, error) {
	status, err := InterfaceOperStatusByIndex(ifIndex)
	if err != nil {
		return false, err
	}
	return status == windows.IfOperStatusUp, nil
}

func pickBestAdapter(candidates []adapterInfo) (adapterInfo, bool) {
	var best adapterInfo
	found := false
	for _, candidate := range candidates {
		if candidate.IfIndex <= 0 {
			continue
		}
		if !found {
			best = candidate
			found = true
			continue
		}
		bestUp := best.OperStatus == windows.IfOperStatusUp
		candUp := candidate.OperStatus == windows.IfOperStatusUp
		if candUp && !bestUp {
			best = candidate
			continue
		}
		if candUp == bestUp && candidate.IfIndex > best.IfIndex {
			best = candidate
		}
	}
	return best, found
}

func adapterDisplayName(adapter adapterInfo) string {
	if adapter.FriendlyName != "" {
		return adapter.FriendlyName
	}
	if adapter.AdapterName != "" {
		return adapter.AdapterName
	}
	return adapter.Description
}

func hasPrefixFold(value string, prefix string) bool {
	if value == "" || prefix == "" {
		return false
	}
	return strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix))
}

func containsAnyFold(value string, fragments []string) bool {
	if value == "" || len(fragments) == 0 {
		return false
	}
	lower := strings.ToLower(value)
	for _, fragment := range fragments {
		if fragment == "" {
			continue
		}
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func normalizeFragments(fragments []string) []string {
	if len(fragments) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(fragments))
	for _, fragment := range fragments {
		trimmed := strings.ToLower(strings.TrimSpace(fragment))
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
