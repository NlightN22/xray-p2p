//go:build windows

package client

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func ensureXrayBinaryPresent(binDir string) error {
	path := filepath.Join(binDir, "xray.exe")
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xray binary missing at %s (copy xray.exe into this directory before running install)", path)
		}
		return fmt.Errorf("inspect xray binary at %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("expected file at %s, found directory", path)
	}
	return nil
}
