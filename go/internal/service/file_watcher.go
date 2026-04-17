package service

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const defaultWatchDebounce = 400 * time.Millisecond

type fileWatcher struct {
	w          *fsnotify.Watcher
	files      map[string]struct{}
	events     chan string
	pending    chan string
	debounce   time.Duration
	lastHashes map[string]string
	timers     map[string]*time.Timer
	once       sync.Once
	mu         sync.Mutex
}

func newFileWatcher(files []string, debounce time.Duration) (*fileWatcher, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if debounce <= 0 {
		debounce = defaultWatchDebounce
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	watcher := &fileWatcher{
		w:          w,
		files:      make(map[string]struct{}, len(files)),
		events:     make(chan string, 16),
		pending:    make(chan string, 16),
		debounce:   debounce,
		lastHashes: make(map[string]string, len(files)),
		timers:     make(map[string]*time.Timer, len(files)),
	}

	parentDirs := make(map[string]struct{})
	for _, file := range files {
		clean := filepath.Clean(strings.TrimSpace(file))
		if clean == "" {
			continue
		}
		watcher.files[clean] = struct{}{}
		parentDirs[filepath.Dir(clean)] = struct{}{}
		if hash, err := hashFile(clean); err != nil {
			watcher.Close()
			return nil, fmt.Errorf("watch file %s: %w", clean, err)
		} else {
			watcher.lastHashes[clean] = hash
		}
	}

	for dir := range parentDirs {
		info, err := os.Stat(dir)
		if err != nil {
			watcher.Close()
			return nil, fmt.Errorf("watch file dir %s: %w", dir, err)
		}
		if !info.IsDir() {
			watcher.Close()
			return nil, fmt.Errorf("watch file dir %s: not a directory", dir)
		}
		if err := w.Add(dir); err != nil {
			watcher.Close()
			return nil, fmt.Errorf("watch file dir %s: %w", dir, err)
		}
	}

	go watcher.run()
	return watcher, nil
}

func (f *fileWatcher) run() {
	defer close(f.events)
	for {
		select {
		case evt, ok := <-f.w.Events:
			if !ok {
				return
			}
			if evt.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			clean := filepath.Clean(evt.Name)
			if _, ok := f.files[clean]; !ok {
				continue
			}
			f.schedule(clean)
		case path := <-f.pending:
			changed, err := f.refreshHash(path)
			if err != nil {
				logging.Warn("service file watcher hash error", "path", path, "err", err)
				continue
			}
			if !changed {
				continue
			}
			select {
			case f.events <- path:
			default:
				// drop event when buffer full
			}
		case err, ok := <-f.w.Errors:
			if !ok {
				return
			}
			logging.Warn("service watcher error", "err", err)
		}
	}
}

func (f *fileWatcher) schedule(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if timer, ok := f.timers[path]; ok {
		timer.Reset(f.debounce)
		return
	}
	f.timers[path] = time.AfterFunc(f.debounce, func() {
		f.mu.Lock()
		delete(f.timers, path)
		f.mu.Unlock()
		select {
		case f.pending <- path:
		default:
		}
	})
}

func (f *fileWatcher) refreshHash(path string) (bool, error) {
	hash, err := hashFile(path)
	if err != nil {
		return false, err
	}
	if hash == "" {
		// Missing files are treated as "no-op" changes. The service layer uses file watchers
		// to detect new/updated requests (such as apply.request), but a file removal should
		// not trigger a restart loop.
		f.lastHashes[path] = ""
		return false, nil
	}
	prev := f.lastHashes[path]
	if prev == hash {
		return false, nil
	}
	f.lastHashes[path] = hash
	return true, nil
}

func (f *fileWatcher) Close() error {
	f.once.Do(func() {
		_ = f.w.Close()
	})
	return nil
}

func (f *fileWatcher) Events() <-chan string {
	return f.events
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:]), nil
}
