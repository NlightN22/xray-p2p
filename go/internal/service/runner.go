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
	// WatchFiles lists specific files to monitor for changes.
	WatchFiles []string
	// WatchDebounce delays file change handling to collapse rapid updates.
	WatchDebounce time.Duration
	// IgnorePaths lists exact paths that should not trigger restarts when changed.
	IgnorePaths []string
	// MaxRestarts overrides the default restart limit. Zero or negative keeps the default.
	MaxRestarts int
	// RestartDelay specifies the pause between restart attempts. Zero defaults to 3 seconds.
	RestartDelay time.Duration
	// MaxWatchRestarts limits watcher-triggered restarts within WatchRestartWindow. Zero disables.
	MaxWatchRestarts int
	// WatchRestartWindow defines the rolling window for MaxWatchRestarts. Zero disables.
	WatchRestartWindow time.Duration
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
	var fileWatcher *fileWatcher
	var err error
	if len(opts.WatchPaths) > 0 {
		watcher, err = newPathWatcher(opts.WatchPaths)
		if err != nil {
			return fmt.Errorf("service %s: watch setup failed: %w", name, err)
		}
		defer watcher.Close()
	}
	if len(opts.WatchFiles) > 0 {
		fileWatcher, err = newFileWatcher(opts.WatchFiles, opts.WatchDebounce)
		if err != nil {
			return fmt.Errorf("service %s: watch file setup failed: %w", name, err)
		}
		defer fileWatcher.Close()
	}

	var watchCh <-chan string
	if watcher != nil {
		watchCh = watcher.Events()
	}
	var fileWatchCh <-chan string
	if fileWatcher != nil {
		fileWatchCh = fileWatcher.Events()
	}

	ignored := make(map[string]struct{}, len(opts.IgnorePaths))
	for _, path := range considerPathList(opts.IgnorePaths) {
		ignored[path] = struct{}{}
	}

	failures := 0
	watchRestarts := make([]time.Time, 0, 8)
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
				if shouldSkipWatchRestart(opts, &watchRestarts) {
					logging.Warn("service restart skipped (watch limiter)", "service", name, "path", path)
					continue
				}
				restarting = true
				restartPath = path
				cancelChild()
				runErr = <-errCh
				break waitLoop
			case path, ok := <-fileWatchCh:
				if !ok {
					fileWatchCh = nil
					continue
				}
				if shouldIgnoreEvent(path, ignored) {
					continue
				}
				if shouldSkipWatchRestart(opts, &watchRestarts) {
					logging.Warn("service restart skipped (watch limiter)", "service", name, "path", path)
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
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	base := filepath.Base(clean)
	if strings.HasSuffix(base, ".lkg") || strings.HasPrefix(base, ".tmp-") {
		return true
	}
	if len(ignored) == 0 {
		return false
	}
	_, skip := ignored[clean]
	return skip
}

func shouldSkipWatchRestart(opts Options, restarts *[]time.Time) bool {
	if opts.MaxWatchRestarts <= 0 || opts.WatchRestartWindow <= 0 {
		return false
	}
	now := time.Now()
	cutoff := now.Add(-opts.WatchRestartWindow)
	kept := (*restarts)[:0]
	for _, ts := range *restarts {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	*restarts = kept
	if len(*restarts) >= opts.MaxWatchRestarts {
		return true
	}
	*restarts = append(*restarts, now)
	return false
}
