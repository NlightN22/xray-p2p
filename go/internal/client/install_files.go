package client

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

func writeEmbeddedFileIfMissing(templates fs.FS, name, dest string, perm os.FileMode) error {
	if _, err := os.Stat(dest); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: stat %s: %w", dest, err)
	}

	content, err := fs.ReadFile(templates, name)
	if err != nil {
		return fmt.Errorf("xp2p: load template %s: %w", name, err)
	}
	if err := os.WriteFile(dest, content, perm); err != nil {
		return fmt.Errorf("xp2p: write template %s: %w", name, err)
	}
	return nil
}
