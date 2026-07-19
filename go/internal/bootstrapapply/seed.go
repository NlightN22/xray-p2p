package bootstrapapply

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/version"
)

type runtimeMetadata struct {
	Version string `json:"version"`
}

// Seed records a fresh apply generation when Desired is newer than Live or
// Live was compiled by a different xp2p version.
func Seed(role, liveConfigDir, desiredExtensionsDir string) (bool, error) {
	desiredConfigPath, err := config.DesiredConfigPathForRole(role)
	if err != nil {
		return false, err
	}
	desiredInfo, err := os.Stat(desiredConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat desired config %s: %w", desiredConfigPath, err)
	}

	liveMetaPath := filepath.Join(filepath.Clean(liveConfigDir), layout.RuntimeMetaFileName)
	compilerChanged, liveMetaKnown, liveMetaInfo, err := liveCompilerChanged(liveMetaPath)
	if err != nil {
		return false, err
	}
	requestExists, err := pathExists(config.ApplyRequestPath())
	if err != nil {
		return false, err
	}
	if requestExists {
		failed, err := failedRequestExists(role)
		if err != nil {
			return false, err
		}
		if !failed || !liveMetaKnown || !compilerChanged {
			return false, nil
		}
	}

	desiredLatest := latestDesiredTime(desiredInfo.ModTime(), desiredExtensionsDir)
	if !compilerChanged && liveMetaInfo != nil && !desiredLatest.After(liveMetaInfo.ModTime()) {
		return false, nil
	}
	if err := apply.RemoveRoleMarkers(config.ApplyRequestPath(), config.ApplyErrorPath(), role); err != nil {
		return false, err
	}
	req, err := apply.NewRequest(role)
	if err != nil {
		return false, err
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		return false, err
	}
	return true, nil
}

func liveCompilerChanged(path string) (bool, bool, os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, false, nil, nil
		}
		return false, false, nil, fmt.Errorf("stat runtime metadata %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false, nil, fmt.Errorf("read runtime metadata %s: %w", path, err)
	}
	var meta runtimeMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return true, false, info, nil
	}
	return meta.Version != version.Current(), true, info, nil
}

func failedRequestExists(role string) (bool, error) {
	req, exists, err := apply.ReadRequestForRole(config.ApplyRequestPath(), role)
	if err != nil || !exists {
		return false, err
	}
	marker, exists, err := apply.ReadError(config.ApplyErrorPath())
	if err != nil || !exists {
		return false, err
	}
	return marker.RequestID != "" && marker.RequestID == req.ID, nil
}

func latestDesiredTime(latest time.Time, dir string) time.Time {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return latest
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", path, err)
}
