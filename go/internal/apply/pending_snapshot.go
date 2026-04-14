package apply

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
)

type PendingSnapshotOptions struct {
	DesiredConfigFile string
	DesiredConfigDir  string
	LiveConfigFile    string
	LiveConfigDir     string
	PendingConfigFile string
	PendingConfigDir  string
	AuditPath         string
}

func SnapshotPendingFromDesired(opts PendingSnapshotOptions) (bool, error) {
	configSrc, configFromLive := chooseConfigSource(opts.DesiredConfigFile, opts.LiveConfigFile)
	dirBase, dirOverlay := chooseDirSources(opts.DesiredConfigDir, opts.LiveConfigDir)
	if configSrc == "" && dirBase == "" && dirOverlay == "" {
		return false, nil
	}

	if err := os.RemoveAll(opts.PendingConfigDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("apply: clear pending dir %s: %w", opts.PendingConfigDir, err)
	}
	if err := os.MkdirAll(opts.PendingConfigDir, 0o755); err != nil {
		return false, fmt.Errorf("apply: create pending dir %s: %w", opts.PendingConfigDir, err)
	}

	written := false
	if dirBase != "" {
		if err := copyDir(dirBase, opts.PendingConfigDir, opts.AuditPath); err != nil {
			return false, err
		}
		written = true
	}
	if configSrc != "" {
		data, err := os.ReadFile(configSrc)
		if err != nil {
			return false, fmt.Errorf("apply: read config %s: %w", configSrc, err)
		}
		if err := os.MkdirAll(filepath.Dir(opts.PendingConfigFile), 0o755); err != nil {
			return false, fmt.Errorf("apply: create pending config dir: %w", err)
		}
		if err := configio.WriteBytes(opts.PendingConfigFile, data, configio.WriteOptions{
			AuditPath:         opts.AuditPath,
			IgnoreAuditErrors: true,
		}); err != nil {
			return false, fmt.Errorf("apply: write pending config %s: %w", opts.PendingConfigFile, err)
		}
		written = true
	}

	if dirOverlay != "" {
		if err := copyDir(dirOverlay, opts.PendingConfigDir, opts.AuditPath); err != nil {
			return false, err
		}
	}
	if configFromLive && opts.DesiredConfigFile != "" {
		if _, err := os.Stat(opts.DesiredConfigFile); err == nil {
			data, err := os.ReadFile(opts.DesiredConfigFile)
			if err != nil {
				return false, fmt.Errorf("apply: read config %s: %w", opts.DesiredConfigFile, err)
			}
			if err := configio.WriteBytes(opts.PendingConfigFile, data, configio.WriteOptions{
				AuditPath:         opts.AuditPath,
				IgnoreAuditErrors: true,
			}); err != nil {
				return false, fmt.Errorf("apply: write pending config %s: %w", opts.PendingConfigFile, err)
			}
			written = true
		}
	}

	return written, nil
}

func chooseConfigSource(desired, live string) (string, bool) {
	if fileExistsLocal(desired) {
		return desired, false
	}
	if fileExistsLocal(live) {
		return live, true
	}
	return "", false
}

func chooseDirSources(desired, live string) (string, string) {
	if dirExistsLocal(live) {
		if dirExistsLocal(desired) {
			return live, desired
		}
		return live, ""
	}
	if dirExistsLocal(desired) {
		return desired, ""
	}
	return "", ""
}

func copyDir(src, dst, auditPath string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("apply: stat %s: %w", src, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("apply: %s is not a directory", src)
	}
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("apply: read file %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("apply: create dir %s: %w", filepath.Dir(target), err)
		}
		if err := configio.WriteBytes(target, data, configio.WriteOptions{
			AuditPath:         auditPath,
			IgnoreAuditErrors: true,
		}); err != nil {
			return fmt.Errorf("apply: write file %s: %w", target, err)
		}
		return nil
	})
}

func fileExistsLocal(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func dirExistsLocal(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
