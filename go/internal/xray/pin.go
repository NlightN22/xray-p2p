package xray

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed pinned.json
var pinnedData []byte

var (
	pinnedOnce sync.Once
	pinnedErr  error
	pinnedVer  string
)

type pinnedConfig struct {
	Version string `json:"version"`
}

// PinnedVersion returns the pinned xray version from distro/xray/pinned.json.
func PinnedVersion() (string, error) {
	pinnedOnce.Do(func() {
		var cfg pinnedConfig
		if err := json.Unmarshal(pinnedData, &cfg); err != nil {
			pinnedErr = fmt.Errorf("xray: parse pinned.json: %w", err)
			return
		}
		pinnedVer = strings.TrimSpace(cfg.Version)
		if pinnedVer == "" {
			pinnedErr = fmt.Errorf("xray: pinned.json missing version")
		}
	})
	if pinnedErr != nil {
		return "", pinnedErr
	}
	return pinnedVer, nil
}
