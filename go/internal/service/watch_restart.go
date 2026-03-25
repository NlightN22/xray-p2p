package service

import (
	"path/filepath"
	"strings"
	"time"
)

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

func watchRestartAllowed(opts Options, restarts *[]time.Time) (bool, time.Time) {
	if opts.MaxWatchRestarts <= 0 || opts.WatchRestartWindow <= 0 {
		return true, time.Time{}
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
		oldest := (*restarts)[0]
		for _, ts := range (*restarts)[1:] {
			if ts.Before(oldest) {
				oldest = ts
			}
		}
		return false, oldest.Add(opts.WatchRestartWindow)
	}
	return true, time.Time{}
}
