package configbundle

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func writeFileFromReader(path string, reader io.Reader, mode os.FileMode) error {
	perm := mode.Perm()
	if perm == 0 {
		perm = 0o644
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("configbundle: write file %s: %w", path, err)
	}
	if _, err := io.Copy(file, reader); err != nil {
		file.Close()
		return fmt.Errorf("configbundle: write file %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("configbundle: close file %s: %w", path, err)
	}
	if perm != 0 {
		_ = os.Chmod(path, perm)
	}
	return nil
}

func normalizeDirMode(mode os.FileMode) os.FileMode {
	perm := mode.Perm()
	if perm == 0 {
		return 0o755
	}
	return perm
}

func validateArchivePath(name string) (string, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return "", nil
	}
	if strings.Contains(cleanName, "\x00") {
		return "", fmt.Errorf("configbundle: invalid archive entry: %s", name)
	}
	if strings.HasPrefix(cleanName, "/") {
		return "", fmt.Errorf("configbundle: invalid absolute entry: %s", name)
	}
	rel := filepath.FromSlash(cleanName)
	if filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", fmt.Errorf("configbundle: invalid absolute entry: %s", name)
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return "", nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("configbundle: invalid archive entry: %s", name)
	}
	return rel, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("configbundle: create parent dir %s: %w", filepath.Dir(dst), err)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("configbundle: open %s: %w", src, err)
	}
	defer in.Close()
	return writeFileFromReader(dst, in, mode)
}
