package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func withRequestLock(requestPath string, fn func() error) error {
	lockDir := filepath.Join(filepath.Dir(requestPath), "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return fmt.Errorf("apply: ensure request lock directory: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(lockDir, "apply-request.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("apply: open request lock: %w", err)
	}
	defer file.Close()
	if err := lockRoleFile(context.Background(), file); err != nil {
		return err
	}
	defer unlockRoleFile(file)
	return fn()
}
