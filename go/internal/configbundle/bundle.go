package configbundle

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Format string

const (
	FormatZip   Format = "zip"
	FormatTarGz Format = "tar.gz"
)

func DefaultArchiveFormat() Format {
	if runtime.GOOS == "windows" {
		return FormatZip
	}
	return FormatTarGz
}

func DetectArchiveFormat(path string) (Format, error) {
	lower := strings.ToLower(strings.TrimSpace(path))
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return FormatZip, nil
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return FormatTarGz, nil
	default:
		return "", fmt.Errorf("configbundle: unsupported archive format: %s", path)
	}
}

func DefaultArchiveName(role string, format Format, now time.Time) string {
	trimmedRole := strings.TrimSpace(strings.ToLower(role))
	if trimmedRole == "" {
		trimmedRole = "bundle"
	}
	ts := now.UTC().Format("20060102-150405")
	base := fmt.Sprintf("xp2p-%s-backup-%s", trimmedRole, ts)
	switch format {
	case FormatZip:
		return base + ".zip"
	case FormatTarGz:
		return base + ".tar.gz"
	default:
		return base
	}
}

func ExportConfigRoot(root, outputPath string) error {
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

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("configbundle: ensure output directory: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("configbundle: create archive %s: %w", outputPath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	switch format {
	case FormatZip:
		writer := zip.NewWriter(file)
		if err := writeZip(cleanRoot, writer); err != nil {
			_ = writer.Close()
			return err
		}
		if err := writer.Close(); err != nil {
			return fmt.Errorf("configbundle: close zip: %w", err)
		}
		return nil
	case FormatTarGz:
		gz := gzip.NewWriter(file)
		tw := tar.NewWriter(gz)
		if err := writeTarGz(cleanRoot, tw); err != nil {
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
		return nil
	default:
		return fmt.Errorf("configbundle: unsupported format %s", format)
	}
}

func ImportConfigRoot(root, inputPath string) error {
	cleanRoot := filepath.Clean(strings.TrimSpace(root))
	if cleanRoot == "" {
		return fmt.Errorf("configbundle: config root is empty")
	}
	cleanInput := strings.TrimSpace(inputPath)
	if cleanInput == "" {
		return fmt.Errorf("configbundle: input path is empty")
	}

	format, err := DetectArchiveFormat(cleanInput)
	if err != nil {
		return err
	}

	parent := filepath.Dir(cleanRoot)
	desiredRootMode := os.FileMode(0o755)
	if info, err := os.Stat(cleanRoot); err == nil {
		desiredRootMode = info.Mode().Perm()
	}
	tempDir, err := os.MkdirTemp(parent, ".xp2p-import-")
	if err != nil {
		return fmt.Errorf("configbundle: create temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()
	if err := os.Chmod(tempDir, desiredRootMode); err != nil {
		return fmt.Errorf("configbundle: set temp root permissions: %w", err)
	}

	if err := extractArchive(cleanInput, tempDir, format); err != nil {
		return err
	}

	backup := ""
	if exists, err := pathExists(cleanRoot); err != nil {
		return err
	} else if exists {
		backup, err = nextBackupPath(cleanRoot)
		if err != nil {
			return err
		}
		if err := os.Rename(cleanRoot, backup); err != nil {
			return fmt.Errorf("configbundle: backup root: %w", err)
		}
	}

	if err := os.Rename(tempDir, cleanRoot); err != nil {
		if backup != "" {
			_ = os.Rename(backup, cleanRoot)
		}
		return fmt.Errorf("configbundle: activate new root: %w", err)
	}

	return nil
}

func writeZip(root string, writer *zip.Writer) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("configbundle: symlink not supported: %s", path)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if info.IsDir() {
			if !strings.HasSuffix(name, "/") {
				name += "/"
			}
			header := &zip.FileHeader{
				Name: name,
			}
			header.SetMode(info.Mode())
			_, err = writer.CreateHeader(header)
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate
		w, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := io.Copy(w, file); err != nil {
			return err
		}
		return nil
	})
}

func writeTarGz(root string, writer *tar.Writer) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("configbundle: symlink not supported: %s", path)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := io.Copy(writer, file); err != nil {
			return err
		}
		return nil
	})
}

func extractArchive(path, dest string, format Format) error {
	switch format {
	case FormatZip:
		return extractZip(path, dest)
	case FormatTarGz:
		return extractTarGz(path, dest)
	default:
		return fmt.Errorf("configbundle: unsupported format %s", format)
	}
}

func extractZip(path, dest string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("configbundle: open zip %s: %w", path, err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		rel, err := validateArchivePath(file.Name)
		if err != nil {
			return err
		}
		if rel == "" {
			continue
		}
		target := filepath.Join(dest, rel)
		if file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") {
			dirMode := normalizeDirMode(file.Mode())
			if err := os.MkdirAll(target, dirMode); err != nil {
				return fmt.Errorf("configbundle: create dir %s: %w", target, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("configbundle: create parent dir %s: %w", filepath.Dir(target), err)
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("configbundle: open file %s: %w", file.Name, err)
		}
		if err := writeFileFromReader(target, rc, file.Mode()); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
	}
	return nil
}

func extractTarGz(path, dest string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("configbundle: open tar.gz %s: %w", path, err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("configbundle: open gzip %s: %w", path, err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("configbundle: read tar %s: %w", path, err)
		}
		rel, err := validateArchivePath(header.Name)
		if err != nil {
			return err
		}
		if rel == "" {
			continue
		}
		target := filepath.Join(dest, rel)
		switch header.Typeflag {
		case tar.TypeDir:
			dirMode := normalizeDirMode(os.FileMode(header.Mode))
			if err := os.MkdirAll(target, dirMode); err != nil {
				return fmt.Errorf("configbundle: create dir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("configbundle: create parent dir %s: %w", filepath.Dir(target), err)
			}
			if err := writeFileFromReader(target, reader, os.FileMode(header.Mode)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("configbundle: unsupported tar entry %s", header.Name)
		}
	}
}

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

func nextBackupPath(root string) (string, error) {
	ts := time.Now().UTC().Format("20060102-150405")
	base := root + ".bak-" + ts
	for i := 0; i < 100; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}
		if exists, err := pathExists(candidate); err != nil {
			return "", err
		} else if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("configbundle: unable to allocate backup path")
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
