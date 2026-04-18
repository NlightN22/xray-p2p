package apply

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func listFilesInDir(dir string) ([]string, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("apply: stat pending dir %s: %w", dir, err)
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == layout.StateDirName {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("apply: list pending dir %s: %w", dir, err)
	}
	return files, nil
}

func removeMissingLiveFiles(liveDir string, pendingFiles []string) error {
	if strings.TrimSpace(liveDir) == "" {
		return nil
	}
	liveFiles, err := listFilesInDir(liveDir)
	if err != nil {
		return err
	}
	if len(liveFiles) == 0 {
		return nil
	}
	keep := make(map[string]struct{}, len(pendingFiles))
	for _, rel := range pendingFiles {
		keep[filepath.Clean(rel)] = struct{}{}
	}
	for _, rel := range liveFiles {
		clean := filepath.Clean(rel)
		if _, ok := keep[clean]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(liveDir, clean)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("apply: remove live file %s: %w", filepath.Join(liveDir, clean), err)
		}
	}
	return removeEmptyDirs(liveDir)
}

func removeEmptyDirs(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("apply: read dir %s: %w", root, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		child := filepath.Join(root, entry.Name())
		if err := removeEmptyDirs(child); err != nil {
			return err
		}
	}
	entries, err = os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("apply: read dir %s: %w", root, err)
	}
	if len(entries) == 0 {
		if err := os.Remove(root); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("apply: remove dir %s: %w", root, err)
		}
	}
	return nil
}

func fileExists(path string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("apply: stat %s: %w", path, err)
	}
	return true, nil
}
