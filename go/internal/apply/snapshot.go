package apply

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func UpdateLastKnownGood(liveRoot, lkgRoot string) error {
	return replaceSnapshot(liveRoot, lkgRoot)
}

func RestoreLiveFromLkg(liveRoot, lkgRoot string) error {
	if _, err := os.Stat(lkgRoot); err != nil {
		return fmt.Errorf("apply: lkg snapshot missing at %s: %w", lkgRoot, err)
	}
	return replaceSnapshot(lkgRoot, liveRoot)
}

func replaceSnapshot(srcRoot, dstRoot string) error {
	parent := filepath.Dir(dstRoot)
	temp := filepath.Join(parent, fmt.Sprintf(".tmp-%d-%d", os.Getpid(), time.Now().UnixNano()))
	if err := os.RemoveAll(temp); err != nil {
		return fmt.Errorf("apply: clear temp snapshot %s: %w", temp, err)
	}
	if err := copySnapshot(srcRoot, temp); err != nil {
		_ = os.RemoveAll(temp)
		return err
	}
	if err := os.RemoveAll(dstRoot); err != nil {
		_ = os.RemoveAll(temp)
		return fmt.Errorf("apply: clear snapshot %s: %w", dstRoot, err)
	}
	if err := os.Rename(temp, dstRoot); err != nil {
		_ = os.RemoveAll(temp)
		return fmt.Errorf("apply: move snapshot %s -> %s: %w", temp, dstRoot, err)
	}
	return nil
}

func copySnapshot(srcRoot, dstRoot string) error {
	info, err := os.Stat(srcRoot)
	if err != nil {
		return fmt.Errorf("apply: stat snapshot %s: %w", srcRoot, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("apply: snapshot root %s is not a directory", srcRoot)
	}
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		return fmt.Errorf("apply: create snapshot dir %s: %w", dstRoot, err)
	}
	return filepath.WalkDir(srcRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dstRoot, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("apply: read snapshot file %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("apply: create snapshot parent %s: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("apply: write snapshot file %s: %w", target, err)
		}
		return nil
	})
}
