package client

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func writeEmbeddedFile(templates fs.FS, name, dest string, perm os.FileMode) error {
	content, err := fs.ReadFile(templates, name)
	if err != nil {
		return fmt.Errorf("xp2p: load template %s: %w", name, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("xp2p: create config directory %s: %w", filepath.Dir(dest), err)
	}
	if err := os.WriteFile(dest, content, perm); err != nil {
		return fmt.Errorf("xp2p: write template %s: %w", dest, err)
	}
	return nil
}

func renderEmbeddedTemplate(templates fs.FS, name, dest string, data any) error {
	content, err := fs.ReadFile(templates, name)
	if err != nil {
		return fmt.Errorf("xp2p: load template %s: %w", name, err)
	}
	tmpl, err := templateFromBytes(name, content)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("xp2p: create config directory %s: %w", filepath.Dir(dest), err)
	}
	file, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("xp2p: create config %s: %w", dest, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	if err := tmpl.Execute(writer, data); err != nil {
		return fmt.Errorf("xp2p: render template %s: %w", name, err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("xp2p: flush config %s: %w", dest, err)
	}
	return nil
}
