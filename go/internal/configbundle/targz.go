package configbundle

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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
