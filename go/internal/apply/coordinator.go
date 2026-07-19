package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WithRoleLock serializes Desired and Live writers for one role. The OS lock
// is released automatically if the process exits.
func WithRoleLock(ctx context.Context, stateRoot, role string, fn func() error) error {
	normalized := strings.TrimSpace(strings.ToLower(role))
	if normalized == "" || normalized == RoleAny {
		return fmt.Errorf("apply: concrete role is required")
	}
	lockDir := filepath.Join(filepath.Clean(stateRoot), "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return fmt.Errorf("apply: ensure lock directory: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(lockDir, normalized+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("apply: open role lock: %w", err)
	}
	defer file.Close()
	if err := lockRoleFile(ctx, file); err != nil {
		return err
	}
	defer unlockRoleFile(file)
	return fn()
}
