package configbundle

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func DefaultDebugArchiveName(role string, format Format, now time.Time) string {
	trimmedRole := strings.TrimSpace(strings.ToLower(role))
	if trimmedRole == "" {
		trimmedRole = "bundle"
	}
	ts := now.UTC().Format("20060102-150405")
	base := fmt.Sprintf("xp2p-%s-debug-%s", trimmedRole, ts)
	switch format {
	case FormatZip:
		return base + ".zip"
	case FormatTarGz:
		return base + ".tar.gz"
	default:
		return base
	}
}

// CreateDebugBundle collects desired inputs + runtime artifacts/logs for troubleshooting.
func CreateDebugBundle(role, configRoot, logRoot, outputPath string) error {
	cleanRoot := filepath.Clean(strings.TrimSpace(configRoot))
	if cleanRoot == "" {
		return fmt.Errorf("configbundle: config root is empty")
	}
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("configbundle: output path is empty")
	}
	info, err := os.Stat(cleanRoot)
	if err != nil {
		return fmt.Errorf("configbundle: read root %s: %w", cleanRoot, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("configbundle: %s is not a directory", cleanRoot)
	}

	format, err := DetectArchiveFormat(outputPath)
	if err != nil {
		return err
	}

	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("configbundle: ensure output directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(outputDir, ".xp2p-debug-*.tmp")
	if err != nil {
		return fmt.Errorf("configbundle: create archive temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	stagingDir, err := os.MkdirTemp(outputDir, ".xp2p-debug-stage-")
	if err != nil {
		return fmt.Errorf("configbundle: create debug staging dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(stagingDir)
	}()

	if err := stageRoleDesiredInputs(role, cleanRoot, stagingDir); err != nil {
		return err
	}
	if err := stageIfPresent(cleanRoot, stagingDir, layout.AuditLogFileName); err != nil {
		return err
	}
	if err := stageIfPresent(cleanRoot, stagingDir, filepath.Join(layout.StateDirName, layout.ApplyRequestFileName)); err != nil {
		return err
	}
	if err := stageIfPresent(cleanRoot, stagingDir, filepath.Join(layout.StateDirName, layout.ApplyErrorFileName)); err != nil {
		return err
	}

	// Include live runtime artifacts when present.
	if err := stageDirIfPresent(cleanRoot, stagingDir, filepath.Join(layout.StateDirName, layout.LiveDirName)); err != nil {
		return err
	}

	if strings.TrimSpace(logRoot) != "" {
		logInfo, statErr := os.Stat(logRoot)
		if statErr == nil && logInfo.IsDir() {
			if err := stageDirInto(stagingDir, logRoot, layout.LogsDirName); err != nil {
				return err
			}
		}
	}

	switch format {
	case FormatZip:
		writer := zip.NewWriter(tmpFile)
		if err := writeZip(stagingDir, writer); err != nil {
			_ = writer.Close()
			return err
		}
		if err := writer.Close(); err != nil {
			return fmt.Errorf("configbundle: close zip: %w", err)
		}
	case FormatTarGz:
		gz := gzip.NewWriter(tmpFile)
		tw := tar.NewWriter(gz)
		if err := writeTarGz(stagingDir, tw); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return err
		}
		if err := tw.Close(); err != nil {
			_ = gz.Close()
			return fmt.Errorf("configbundle: close tar: %w", err)
		}
		if err := gz.Close(); err != nil {
			return fmt.Errorf("configbundle: close gzip: %w", err)
		}
	default:
		return fmt.Errorf("configbundle: unsupported format %s", format)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("configbundle: close archive temp file: %w", err)
	}
	if err := os.Remove(outputPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("configbundle: remove existing archive %s: %w", outputPath, err)
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return fmt.Errorf("configbundle: finalize archive %s: %w", outputPath, err)
	}
	return nil
}

func stageDirInto(stagingRoot, sourceDir, targetRel string) error {
	targetAbs := filepath.Join(stagingRoot, targetRel)
	if err := os.MkdirAll(targetAbs, 0o755); err != nil {
		return fmt.Errorf("configbundle: create %s: %w", targetAbs, err)
	}
	return filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("configbundle: symlink not supported: %s", path)
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		dst := filepath.Join(targetAbs, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, normalizeDirMode(info.Mode()))
		}
		return copyFile(path, dst, info.Mode())
	})
}
