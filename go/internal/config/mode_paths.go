package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func resolveConfigPath(explicit, role string) (string, error) {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return filepath.Clean(trimmed), nil
	}
	if strings.EqualFold(strings.TrimSpace(role), "client") {
		return filepath.Clean(PendingConfigPath(layout.ClientConfigFileName)), nil
	}
	if strings.EqualFold(strings.TrimSpace(role), "server") {
		return filepath.Clean(PendingConfigPath(layout.ServerConfigFileName)), nil
	}
	return "", fmt.Errorf("config: unsupported role %q", role)
}
