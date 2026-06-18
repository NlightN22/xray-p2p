package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func loadLiveRuntimeMeta(liveConfigDir string) (runtimeMeta, error) {
	path := filepath.Join(filepath.Clean(strings.TrimSpace(liveConfigDir)), layout.RuntimeMetaFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return runtimeMeta{}, fmt.Errorf("runtime metadata missing at %s", path)
		}
		return runtimeMeta{}, fmt.Errorf("read runtime metadata %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return runtimeMeta{}, fmt.Errorf("runtime metadata is empty at %s", path)
	}
	var meta runtimeMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return runtimeMeta{}, fmt.Errorf("parse runtime metadata %s: %w", path, err)
	}
	meta.Role = strings.TrimSpace(meta.Role)
	meta.TunName = strings.TrimSpace(meta.TunName)
	meta.TunAddr = strings.TrimSpace(meta.TunAddr)
	meta.TunMode = strings.TrimSpace(meta.TunMode)
	meta.FullTag = strings.TrimSpace(meta.FullTag)
	normalizeRuntimeXrayAssets(&meta.XrayAssets)
	return meta, nil
}

func normalizeRuntimeXrayAssets(cfg *config.XrayAssetsConfig) {
	cfg.StaleAfter = strings.TrimSpace(cfg.StaleAfter)
	for i := range cfg.Files {
		cfg.Files[i].Name = strings.TrimSpace(cfg.Files[i].Name)
		cfg.Files[i].URL = strings.TrimSpace(cfg.Files[i].URL)
		cfg.Files[i].StaleAfter = strings.TrimSpace(cfg.Files[i].StaleAfter)
	}
}
