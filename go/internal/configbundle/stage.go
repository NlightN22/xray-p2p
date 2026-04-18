package configbundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func stageIfPresent(root, staging, rel string) error {
	src := filepath.Join(root, rel)
	info, err := os.Stat(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("configbundle: stat %s: %w", src, err)
	}
	if info.IsDir() {
		return nil
	}
	dst := filepath.Join(staging, rel)
	return copyFile(src, dst, info.Mode())
}

func stageDirIfPresent(root, staging, rel string) error {
	src := filepath.Join(root, rel)
	info, err := os.Stat(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("configbundle: stat %s: %w", src, err)
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("configbundle: symlink not supported: %s", path)
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(staging, relPath)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.MkdirAll(target, normalizeDirMode(info.Mode()))
		}
		return copyFile(path, target, info.Mode())
	})
}

func stageJSONDir(root, staging, rel string) error {
	src := filepath.Join(root, rel)
	info, err := os.Stat(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("configbundle: stat %s: %w", src, err)
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("configbundle: symlink not supported: %s", path)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			return os.MkdirAll(filepath.Join(staging, relPath), normalizeDirMode(info.Mode()))
		}
		if strings.ToLower(filepath.Ext(d.Name())) != ".json" {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return copyFile(path, filepath.Join(staging, relPath), info.Mode())
	})
}
