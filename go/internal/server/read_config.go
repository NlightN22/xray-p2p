package server

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func readConfigWithFallback(primaryPath, fallbackPath string) ([]byte, error) {
	if strings.TrimSpace(primaryPath) != "" {
		if data, err := os.ReadFile(primaryPath); err == nil {
			return data, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read %s: %w", primaryPath, err)
		}
	}
	if strings.TrimSpace(fallbackPath) == "" {
		return nil, fmt.Errorf("config path is empty")
	}
	data, err := os.ReadFile(fallbackPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", fallbackPath, err)
	}
	return data, nil
}
