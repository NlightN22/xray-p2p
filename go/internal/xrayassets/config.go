package xrayassets

import (
	"fmt"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func FromConfig(cfg config.XrayAssetsConfig) (Config, error) {
	defaultStaleAfter, err := parseDuration(cfg.StaleAfter)
	if err != nil {
		return Config{}, fmt.Errorf("xray_assets.stale_after: %w", err)
	}
	files := make([]File, 0, len(cfg.Files))
	for _, item := range cfg.Files {
		if err := ValidateName(item.Name); err != nil {
			return Config{}, err
		}
		staleAfter := defaultStaleAfter
		if strings.TrimSpace(item.StaleAfter) != "" {
			staleAfter, err = parseDuration(item.StaleAfter)
			if err != nil {
				return Config{}, fmt.Errorf("xray_assets.files[%s].stale_after: %w", item.Name, err)
			}
		}
		files = append(files, File{
			Name:       strings.TrimSpace(item.Name),
			URL:        strings.TrimSpace(item.URL),
			StaleAfter: staleAfter,
		})
	}
	return Config{Files: files}, nil
}

func parseDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return 0, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if d < 0 {
		return 0, fmt.Errorf("duration must not be negative")
	}
	return d, nil
}
