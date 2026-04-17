package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	return meta, nil
}
