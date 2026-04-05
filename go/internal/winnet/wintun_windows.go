//go:build windows

package winnet

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type WintunCleanupResult string

const (
	WintunCleanupClosed   WintunCleanupResult = "closed"
	WintunCleanupNotFound WintunCleanupResult = "not_found"
	WintunCleanupSkipped  WintunCleanupResult = "skipped"
)

func CleanupWintunAdapter(wintunPath string, adapterName string) (WintunCleanupResult, error) {
	name := strings.TrimSpace(adapterName)
	if name == "" {
		return WintunCleanupSkipped, nil
	}
	path := strings.TrimSpace(wintunPath)
	if path == "" {
		return WintunCleanupSkipped, errors.New("xp2p: wintun dll path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		return WintunCleanupSkipped, fmt.Errorf("xp2p: wintun dll not found at %s: %w", path, err)
	}
	dll := windows.NewLazyDLL(path)
	if err := dll.Load(); err != nil {
		return WintunCleanupSkipped, fmt.Errorf("xp2p: load wintun dll: %w", err)
	}
	openProc, err := dll.FindProc("WintunOpenAdapter")
	if err != nil {
		return WintunCleanupSkipped, fmt.Errorf("xp2p: find WintunOpenAdapter: %w", err)
	}
	closeProc, err := dll.FindProc("WintunCloseAdapter")
	if err != nil {
		return WintunCleanupSkipped, fmt.Errorf("xp2p: find WintunCloseAdapter: %w", err)
	}

	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return WintunCleanupSkipped, fmt.Errorf("xp2p: encode adapter name: %w", err)
	}

	handle, _, callErr := openProc.Call(uintptr(unsafe.Pointer(namePtr)))
	if handle == 0 {
		err := resolveWintunCallError(callErr)
		if err == nil || errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return WintunCleanupNotFound, nil
		}
		return WintunCleanupNotFound, fmt.Errorf("xp2p: open wintun adapter: %w", err)
	}
	closeProc.Call(handle)
	return WintunCleanupClosed, nil
}

func resolveWintunCallError(callErr error) error {
	if callErr == nil {
		if last := windows.GetLastError(); last != syscall.Errno(0) {
			return last
		}
		return nil
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno == 0 {
		if last := windows.GetLastError(); last != syscall.Errno(0) {
			return last
		}
		return nil
	}
	return callErr
}
