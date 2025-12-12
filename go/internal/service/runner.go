package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

// MaxRestartAttempts defines the default limit of restart retries after failures.
const MaxRestartAttempts = 5

// Options controls service runner behaviour.
type Options struct {
	// Name is used for log messages.
	Name string
	// WatchPaths lists directories whose modifications should trigger graceful restarts.
	WatchPaths []string
	// IgnorePaths lists exact paths that should not trigger restarts when changed.
	IgnorePaths []string
	// MaxRestarts overrides the default restart limit. Zero or negative keeps the default.
	MaxRestarts int
	// RestartDelay specifies the pause between restart attempts. Zero defaults to 3 seconds.
	RestartDelay time.Duration
}

// Run launches the supplied function, restarts it on failure, and watches the configured paths.
func Run(ctx context.Context, opts Options, run func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if run == nil {
		return fmt.Errorf("service %s: missing run callback", opts.Name)
	}

	name := opts.Name
	if name == "" {
		name = "service"
	}

	maxRestarts := opts.MaxRestarts
	if maxRestarts <= 0 {
		maxRestarts = MaxRestartAttempts
	}

	restartDelay := opts.RestartDelay
	if restartDelay <= 0 {
		restartDelay = 3 * time.Second
	}

	var watcher *pathWatcher
	var err error
	if len(opts.WatchPaths) > 0 {
		watcher, err = newPathWatcher(opts.WatchPaths)
		if err != nil {
			return fmt.Errorf("service %s: watch setup failed: %w", name, err)
		}
		defer watcher.Close()
	}

	var watchCh <-chan string
	if watcher != nil {
		watchCh = watcher.Events()
	}

	ignored := make(map[string]struct{}, len(opts.IgnorePaths))
	for _, path := range considerPathList(opts.IgnorePaths) {
		ignored[path] = struct{}{}
	}

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		childCtx, cancelChild := context.WithCancel(ctx)
		errCh := make(chan error, 1)
		go func() {
			errCh <- run(childCtx)
		}()

		var runErr error
		restarting := false
		restartPath := ""

	waitLoop:
		for {
			select {
			case <-ctx.Done():
				cancelChild()
				<-errCh
				return nil
			case err := <-errCh:
				runErr = err
				break waitLoop
			case path, ok := <-watchCh:
				if !ok {
					watchCh = nil
					continue
				}
				if shouldIgnoreEvent(path, ignored) {
					continue
				}
				restarting = true
				restartPath = path
				cancelChild()
				runErr = <-errCh
				break waitLoop
			}
		}

		cancelChild()

		if restarting {
			failures = 0
			if restartPath != "" {
				logging.Info("service configuration change detected", "service", name, "path", restartPath)
			} else {
				logging.Info("service restart requested", "service", name)
			}
			if !waitWithContext(ctx, restartDelay) {
				return nil
			}
			continue
		}

		if runErr != nil {
			failures++
			logging.Warn("service run failed", "service", name, "attempt", failures, "max_attempts", maxRestarts, "err", runErr)
			if failures >= maxRestarts {
				return fmt.Errorf("service %s exceeded restart limit (%d): %w", name, maxRestarts, runErr)
			}
			if !waitWithContext(ctx, restartDelay) {
				return nil
			}
			continue
		}

		logging.Info("service stopped cleanly", "service", name)
		return nil
	}
}

func waitWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func considerPathList(paths []string) []string {
	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, filepath.Clean(trimmed))
	}
	return cleaned
}

func shouldIgnoreEvent(path string, ignored map[string]struct{}) bool {
	if len(ignored) == 0 {
		return false
	}
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	_, skip := ignored[clean]
	return skip
}
