//go:build windows

package preflight

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

type windowsTunPreflight struct{}

var (
	osStatFunc          = os.Stat
	wintunLoadCheckFunc = checkWintunLoadable
)

func (windowsTunPreflight) Check(ctx context.Context, cfg TunConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}

	path := strings.TrimSpace(cfg.WintunDLLPath)
	if path == "" {
		return ErrTunUnavailable{
			OS:     "windows",
			Reason: "wintun dll path is not configured",
			Hint:   "place wintun.dll next to xray.exe (bin directory) and retry",
		}
	}
	if info, err := osStatFunc(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrTunUnavailable{
				OS:     "windows",
				Reason: fmt.Sprintf("wintun.dll not found at %s", path),
				Hint:   "place wintun.dll next to xray.exe (bin directory) and retry",
			}
		}
		return ErrTunUnavailable{
			OS:     "windows",
			Reason: fmt.Sprintf("inspect wintun.dll at %s failed (%v)", path, err),
			Hint:   "ensure wintun.dll is present and readable",
		}
	} else if info.IsDir() {
		return ErrTunUnavailable{
			OS:     "windows",
			Reason: fmt.Sprintf("expected file at %s, found directory", path),
			Hint:   "place wintun.dll next to xray.exe (bin directory) and retry",
		}
	}

	if err := wintunLoadCheckFunc(path); err != nil {
		return ErrTunUnavailable{
			OS:     "windows",
			Reason: fmt.Sprintf("wintun.dll cannot be loaded (%v)", err),
			Hint:   "replace wintun.dll with a compatible version",
		}
	}
	return nil
}

func checkWintunLoadable(path string) error {
	dll := windows.NewLazyDLL(path)
	if err := dll.Load(); err != nil {
		return err
	}
	required := []string{
		"WintunCreateAdapter",
		"WintunOpenAdapter",
		"WintunCloseAdapter",
	}
	for _, name := range required {
		if err := dll.NewProc(name).Find(); err != nil {
			return fmt.Errorf("missing export %s", name)
		}
	}
	return nil
}

func defaultWintunPath(installDir string) string {
	if strings.TrimSpace(installDir) == "" {
		return ""
	}
	return filepath.Join(installDir, "bin", "wintun.dll")
}
