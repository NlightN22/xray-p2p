package configbundle

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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
