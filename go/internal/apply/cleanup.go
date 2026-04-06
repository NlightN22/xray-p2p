package apply

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// CleanupPending removes pending config artifacts after successful apply.
func CleanupPending(set PendingSet) error {
	if err := removeFileIfExists(set.PendingConfigFile); err != nil {
		return err
	}
	if err := removeDirIfExists(set.PendingConfigDir); err != nil {
		return err
	}
	return nil
}

func removeFileIfExists(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("apply: remove pending file %s: %w", path, err)
	}
	return nil
}

func removeDirIfExists(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("apply: remove pending dir %s: %w", path, err)
	}
	return nil
}
