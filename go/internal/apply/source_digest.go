package apply

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SourceDigest identifies the complete Desired input set used by a compiler.
func SourceDigest(configPath, extensionsDir string) (string, error) {
	paths := []string{filepath.Clean(configPath)}
	if strings.TrimSpace(extensionsDir) != "" {
		err := filepath.WalkDir(filepath.Clean(extensionsDir), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				if errors.Is(walkErr, os.ErrNotExist) {
					return nil
				}
				return walkErr
			}
			if !entry.IsDir() {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("apply: list desired inputs: %w", err)
		}
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		if _, err := io.WriteString(hash, filepath.ToSlash(path)+"\x00"); err != nil {
			return "", err
		}
		file, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				_, _ = io.WriteString(hash, "missing\x00")
				continue
			}
			return "", fmt.Errorf("apply: read desired input %s: %w", path, err)
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		_, _ = io.WriteString(hash, "\x00")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
