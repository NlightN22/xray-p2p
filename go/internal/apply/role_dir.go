package apply

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func UpdateRoleLastKnownGood(liveDir, lkgDir string) error {
	if _, err := os.Stat(liveDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("apply: stat live dir %s: %w", liveDir, err)
	}
	return replaceSnapshot(liveDir, lkgDir)
}

func RestoreRoleLiveFromLkg(liveDir, lkgDir string) error {
	if _, err := os.Stat(lkgDir); err != nil {
		return fmt.Errorf("apply: lkg snapshot missing at %s: %w", lkgDir, err)
	}
	return replaceSnapshot(lkgDir, liveDir)
}

func ReplaceRoleLiveDir(liveDir, lkgDir string, files map[string][]byte) error {
	if liveDir == "" {
		return fmt.Errorf("apply: live dir is empty")
	}
	parent := filepath.Dir(liveDir)
	tempDir := filepath.Join(parent, fmt.Sprintf(".tmp-role-%d-%d", os.Getpid(), time.Now().UnixNano()))
	backupDir := filepath.Join(parent, fmt.Sprintf(".bak-role-%d-%d", os.Getpid(), time.Now().UnixNano()))
	if err := os.RemoveAll(tempDir); err != nil {
		return fmt.Errorf("apply: clear temp dir %s: %w", tempDir, err)
	}
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return fmt.Errorf("apply: create temp dir %s: %w", tempDir, err)
	}
	for name, data := range files {
		target := filepath.Join(tempDir, filepath.Clean(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			_ = os.RemoveAll(tempDir)
			return fmt.Errorf("apply: create artifact dir %s: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			_ = os.RemoveAll(tempDir)
			return fmt.Errorf("apply: write artifact %s: %w", target, err)
		}
	}

	if err := UpdateRoleLastKnownGood(liveDir, lkgDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return err
	}

	if err := os.RemoveAll(backupDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.RemoveAll(tempDir)
		return fmt.Errorf("apply: clear backup dir %s: %w", backupDir, err)
	}

	backupCreated := false
	if _, err := os.Stat(liveDir); err == nil {
		if err := os.Rename(liveDir, backupDir); err != nil {
			_ = os.RemoveAll(tempDir)
			return fmt.Errorf("apply: backup live dir %s: %w", liveDir, err)
		}
		backupCreated = true
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.RemoveAll(tempDir)
		return fmt.Errorf("apply: stat live dir %s: %w", liveDir, err)
	}

	if err := os.Rename(tempDir, liveDir); err != nil {
		_ = os.RemoveAll(tempDir)
		if backupCreated {
			_ = os.Rename(backupDir, liveDir)
		}
		_ = RestoreRoleLiveFromLkg(liveDir, lkgDir)
		return fmt.Errorf("apply: activate live dir %s: %w", liveDir, err)
	}
	if backupCreated {
		_ = os.RemoveAll(backupDir)
	}
	return nil
}
