package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type pathWatcher struct {
	w      *fsnotify.Watcher
	paths  []string
	events chan string
	once   sync.Once
}

func newPathWatcher(paths []string) (*pathWatcher, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	watcher := &pathWatcher{
		w:      w,
		events: make(chan string, 16),
	}

	for _, path := range uniquePaths(paths) {
		if strings.TrimSpace(path) == "" {
			continue
		}
		clean := filepath.Clean(path)
		info, err := os.Stat(clean)
		if err != nil {
			watcher.Close()
			return nil, fmt.Errorf("watch path %s: %w", clean, err)
		}
		if !info.IsDir() {
			watcher.Close()
			return nil, fmt.Errorf("watch path %s: not a directory", clean)
		}
		if err := w.Add(clean); err != nil {
			watcher.Close()
			return nil, fmt.Errorf("watch path %s: %w", clean, err)
		}
		watcher.paths = append(watcher.paths, clean)
	}

	go watcher.run()
	return watcher, nil
}

func (p *pathWatcher) run() {
	defer close(p.events)
	for {
		select {
		case evt, ok := <-p.w.Events:
			if !ok {
				return
			}
			if evt.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			clean := filepath.Clean(evt.Name)
			select {
			case p.events <- clean:
			default:
				// drop event when buffer full
			}
		case err, ok := <-p.w.Errors:
			if !ok {
				return
			}
			logging.Warn("service watcher error", "err", err)
		}
	}
}

func (p *pathWatcher) Close() error {
	p.once.Do(func() {
		_ = p.w.Close()
	})
	return nil
}

func (p *pathWatcher) Events() <-chan string {
	return p.events
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}
