package ui

import (
	"context"
	"sync"
)

var wailsContext struct {
	mu  sync.RWMutex
	ctx context.Context
}

func SetWailsContext(ctx context.Context) {
	wailsContext.mu.Lock()
	wailsContext.ctx = ctx
	wailsContext.mu.Unlock()
}

func ensureWailsContext() (context.Context, bool) {
	wailsContext.mu.RLock()
	defer wailsContext.mu.RUnlock()
	if wailsContext.ctx == nil {
		return nil, false
	}
	return wailsContext.ctx, true
}
