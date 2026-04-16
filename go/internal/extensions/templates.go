package extensions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func EnsureTemplates(dir string) error {
	if dir == "" {
		return fmt.Errorf("extensions: dir is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("extensions: create dir %s: %w", dir, err)
	}

	templates := map[string][]byte{
		RoutingAfterSystemFile:  []byte("{\"rules\":[]}\n"),
		RoutingAfterManagedFile: []byte("{\"rules\":[]}\n"),
		InboundsAppendFile:      []byte("{\"inbounds\":[]}\n"),
		OutboundsAppendFile:     []byte("{\"outbounds\":[]}\n"),
	}
	for name, contents := range templates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("extensions: stat %s: %w", path, err)
		}
		if err := os.WriteFile(path, contents, 0o644); err != nil {
			return fmt.Errorf("extensions: write %s: %w", path, err)
		}
	}
	return nil
}

