package xrayassets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func AssetDirForXrayPath(xrayPath string) string {
	if env := strings.TrimSpace(os.Getenv("XRAY_LOCATION_ASSET")); env != "" {
		return filepath.Clean(env)
	}
	return filepath.Dir(filepath.Clean(xrayPath))
}

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("asset name is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) || strings.Contains(name, "..") {
		return fmt.Errorf("asset name %q must be a safe basename", name)
	}
	if filepath.IsAbs(name) || filepath.Base(name) != name {
		return fmt.Errorf("asset name %q must be a safe basename", name)
	}
	if !strings.HasSuffix(strings.ToLower(name), ".dat") {
		return fmt.Errorf("asset name %q must use .dat extension", name)
	}
	return nil
}

func safePath(assetDir, name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	dir, err := filepath.Abs(filepath.Clean(strings.TrimSpace(assetDir)))
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", fmt.Errorf("asset directory is required")
	}
	path := filepath.Join(dir, strings.TrimSpace(name))
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("asset path escapes asset directory: %s", name)
	}
	return path, nil
}
