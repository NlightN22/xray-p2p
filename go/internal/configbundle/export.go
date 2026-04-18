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

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func ExportConfigRoot(root, outputPath string) error {
	return ExportRoleConfigRoot("any", root, outputPath)
}

func ExportRoleConfigRoot(role, root, outputPath string) error {
	cleanRoot := filepath.Clean(strings.TrimSpace(root))
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

	tmpFile, err := os.CreateTemp(outputDir, ".xp2p-export-*.tmp")
	if err != nil {
		return fmt.Errorf("configbundle: create archive temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	stagingDir, err := os.MkdirTemp(outputDir, ".xp2p-export-stage-")
	if err != nil {
		return fmt.Errorf("configbundle: create export staging dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(stagingDir)
	}()

	if err := stageRoleDesiredInputs(role, cleanRoot, stagingDir); err != nil {
		return err
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

func exportConfigFileIfPresent(root, staging, name string) error {
	src := filepath.Join(root, name)
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
	dst := filepath.Join(staging, name)
	return copyFile(src, dst, info.Mode())
}

func exportRoleConfigDir(root, staging, dir string) error {
	return stageJSONDir(root, staging, dir)
}

func exportRoleTLSDir(root, staging, rel string) error {
	return stageDirIfPresent(root, staging, rel)
}

func stageRoleDesiredInputs(role, root, staging string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = "any"
	}

	includeClient := role == "client" || role == "any"
	includeServer := role == "server" || role == "any"

	if includeClient {
		if err := exportConfigFileIfPresent(root, staging, layout.ClientConfigFileName); err != nil {
			return err
		}
		if err := exportRoleConfigDir(root, staging, layout.ClientConfigDir); err != nil {
			return err
		}
	}
	if includeServer {
		if err := exportConfigFileIfPresent(root, staging, layout.ServerConfigFileName); err != nil {
			return err
		}
		if err := exportRoleConfigDir(root, staging, layout.ServerConfigDir); err != nil {
			return err
		}
		if err := exportRoleTLSDir(root, staging, filepath.Join("tls", "server")); err != nil {
			return err
		}
	}
	return nil
}
