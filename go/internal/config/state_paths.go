package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func PendingConfigDir(configDir string) (string, error) {
	return stateDirForConfigDir(configDir, PendingRoot())
}

func LiveConfigDir(configDir string) (string, error) {
	return stateDirForConfigDir(configDir, LiveRoot())
}

func LkgConfigDir(configDir string) (string, error) {
	return stateDirForConfigDir(configDir, LkgRoot())
}

func stateDirForConfigDir(configDir, stateRoot string) (string, error) {
	root := filepath.Clean(ConfigRoot())
	dir := filepath.Clean(configDir)
	if root == "." || root == "" {
		return filepath.Join(stateRoot, dir), nil
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return "", fmt.Errorf("config: resolve state dir: %w", err)
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return stateRoot, nil
	}
	if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", fmt.Errorf("config: config dir %s is outside config root %s", dir, root)
	}
	return filepath.Join(stateRoot, rel), nil
}

func PendingConfigDirFromLive(liveDir string) (string, error) {
	return stateDirFromStateDir(liveDir, LiveRoot(), PendingRoot())
}

func LiveConfigDirFromPending(pendingDir string) (string, error) {
	return stateDirFromStateDir(pendingDir, PendingRoot(), LiveRoot())
}

func stateDirFromStateDir(dir, fromRoot, toRoot string) (string, error) {
	from := filepath.Clean(fromRoot)
	absolute := filepath.Clean(dir)
	rel, err := filepath.Rel(from, absolute)
	if err != nil {
		return "", fmt.Errorf("config: resolve state dir: %w", err)
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return toRoot, nil
	}
	if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", fmt.Errorf("config: state dir %s is outside state root %s", absolute, from)
	}
	return filepath.Join(toRoot, rel), nil
}
