package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type LogWatchOptions struct {
	Name          string
	Paths         []string
	Files         []string
	IgnorePaths   []string
	IgnorePrefix  []string
	WatchDebounce time.Duration
}

func StartLogWatcher(ctx context.Context, opts LogWatchOptions) (StopFunc, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	watchPaths := considerPathList(opts.Paths)
	watchFiles := considerPathList(opts.Files)
	if len(watchPaths) == 0 && len(watchFiles) == 0 {
		return noopStop, nil
	}

	var pathWatcher *pathWatcher
	var fileWatcher *fileWatcher
	var err error

	if len(watchPaths) > 0 {
		pathWatcher, err = newPathWatcher(watchPaths)
		if err != nil {
			return nil, err
		}
	}
	if len(watchFiles) > 0 {
		fileWatcher, err = newFileWatcher(watchFiles, opts.WatchDebounce)
		if err != nil {
			if pathWatcher != nil {
				_ = pathWatcher.Close()
			}
			return nil, err
		}
	}

	ignored := make(map[string]struct{}, len(opts.IgnorePaths))
	for _, path := range considerPathList(opts.IgnorePaths) {
		ignored[path] = struct{}{}
	}
	prefixes := make([]string, 0, len(opts.IgnorePrefix))
	for _, prefix := range opts.IgnorePrefix {
		clean := filepath.Clean(strings.TrimSpace(prefix))
		if clean == "" {
			continue
		}
		prefixes = append(prefixes, clean)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	var stopOnce sync.Once
	go func() {
		defer close(done)
		defer func() {
			if pathWatcher != nil {
				_ = pathWatcher.Close()
			}
			if fileWatcher != nil {
				_ = fileWatcher.Close()
			}
		}()
		var pathCh <-chan string
		var fileCh <-chan string
		if pathWatcher != nil {
			pathCh = pathWatcher.Events()
		}
		if fileWatcher != nil {
			fileCh = fileWatcher.Events()
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case path, ok := <-pathCh:
				if !ok {
					pathCh = nil
					continue
				}
				if shouldIgnoreLogEvent(path, ignored, prefixes) {
					continue
				}
				logging.Warn("service configuration change observed", "service", opts.Name, "path", path)
			case path, ok := <-fileCh:
				if !ok {
					fileCh = nil
					continue
				}
				if shouldIgnoreLogEvent(path, ignored, prefixes) {
					continue
				}
				logging.Warn("service configuration change observed", "service", opts.Name, "path", path)
			}
		}
	}()

	return func(ctx context.Context) error {
		stopOnce.Do(func() { close(stop) })
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("log watcher shutdown: %w", ctx.Err())
		}
	}, nil
}

func shouldIgnoreLogEvent(path string, ignored map[string]struct{}, prefixes []string) bool {
	if shouldIgnoreEvent(path, ignored) {
		return true
	}
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	for _, prefix := range prefixes {
		if clean == prefix {
			return true
		}
		rel, err := filepath.Rel(prefix, clean)
		if err != nil {
			continue
		}
		if rel != "." && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
}
