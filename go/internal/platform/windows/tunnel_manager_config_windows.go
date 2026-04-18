//go:build windows

package windows

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func loadConfigForRole(path string, role string) (config.Config, error) {
	readPath := resolveConfigReadPath(path, role)
	return config.Load(config.Options{
		Path:         readPath,
		AllowInvalid: true,
	})
}

func resolveConfigReadPath(explicit string, role string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	filename := layout.ClientConfigFileName
	if strings.EqualFold(role, "server") {
		filename = layout.ServerConfigFileName
	}
	live := config.ConfigPath(filename)
	if _, err := os.Stat(live); err == nil {
		return live
	}
	return config.PendingConfigPath(filename)
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	if parsed, err := time.ParseDuration(trimmed); err == nil {
		return parsed
	}
	if seconds, err := parseInt(trimmed); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}

func parseInt(value string) (int, error) {
	out, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
