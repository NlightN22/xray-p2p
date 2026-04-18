//go:build windows

package server

import (
	"errors"
	"fmt"
	"os"
)

func ensureXrayBinaryPresent(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xray binary is missing at %s (copy xray.exe into this directory before running install)", path)
		}
		return fmt.Errorf("inspect xray binary at %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("expected file at %s, found directory", path)
	}
	return nil
}
