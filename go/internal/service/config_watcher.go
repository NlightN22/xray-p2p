package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type ConfigWatchOptions struct {
	Paths         []string
	Files         []string
	IgnorePaths   []string
	IgnorePrefix  []string
	WatchDebounce time.Duration
	OnChange      func(context.Context, string)
}

type StopFunc func(context.Context) error

func noopStop(context.Context) error { return nil }

func StartConfigWatcher(ctx context.Context, opts ConfigWatchOptions) (StopFunc, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.OnChange == nil {
		return noopStop, nil
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
	callbackCtx, cancelCallbacks := context.WithCancel(ctx)
	var stopOnce sync.Once
	var callbacks sync.WaitGroup
	var running int32
	var rerun int32
	var mu sync.Mutex
	trigger := func(path string) {
		mu.Lock()
		defer mu.Unlock()
		if !atomic.CompareAndSwapInt32(&running, 0, 1) {
			atomic.StoreInt32(&rerun, 1)
			return
		}
		callbacks.Add(1)
		go func(first string) {
			defer callbacks.Done()
			defer atomic.StoreInt32(&running, 0)
			opts.OnChange(callbackCtx, first)
			if atomic.SwapInt32(&rerun, 0) == 1 {
				opts.OnChange(callbackCtx, first)
			}
		}(path)
	}

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
				logging.Debug("service config change detected", "path", path)
				trigger(path)
			case path, ok := <-fileCh:
				if !ok {
					fileCh = nil
					continue
				}
				if shouldIgnoreLogEvent(path, ignored, prefixes) {
					continue
				}
				logging.Debug("service config change detected", "path", path)
				trigger(path)
			}
		}
	}()
	allDone := make(chan struct{})
	go func() {
		<-done
		callbacks.Wait()
		close(allDone)
	}()

	return func(ctx context.Context) error {
		stopOnce.Do(func() { close(stop) })
		cancelCallbacks()
		select {
		case <-allDone:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("config watcher shutdown: %w", ctx.Err())
		}
	}, nil
}
