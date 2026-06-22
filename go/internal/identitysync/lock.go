package identitysync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

type OperationLock struct {
	path string
}

func DefaultOperationLock() OperationLock {
	return OperationLock{path: filepath.Join(config.IdentityStateDir(), "operation.lock")}
}

func (l OperationLock) With(ctx context.Context, fn func() error) error {
	release, err := l.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	return fn()
}

func (l OperationLock) Acquire(ctx context.Context) (func(), error) {
	path := filepath.Clean(l.path)
	if path == "." || path == "" {
		return nil, errors.New("identity operation lock path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("ensure identity lock dir: %w", err)
	}
	for {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, _ = file.WriteString(time.Now().UTC().Format(time.RFC3339Nano) + "\n")
			_ = file.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire identity operation lock: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
