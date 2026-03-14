package ui

import (
	"context"
	"sync"
	"time"
)

var wailsContext struct {
	mu      sync.RWMutex
	ctx     context.Context
	readyCh chan struct{}
}

func SetWailsContext(ctx context.Context) {
	wailsContext.mu.Lock()
	wailsContext.ctx = ctx
	if wailsContext.readyCh == nil {
		wailsContext.readyCh = make(chan struct{})
	}
	select {
	case <-wailsContext.readyCh:
	default:
		close(wailsContext.readyCh)
	}
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

func waitWailsContext(timeout time.Duration) (context.Context, bool) {
	wailsContext.mu.RLock()
	ctx := wailsContext.ctx
	readyCh := wailsContext.readyCh
	wailsContext.mu.RUnlock()
	if ctx != nil {
		return ctx, true
	}
	if readyCh == nil {
		wailsContext.mu.Lock()
		if wailsContext.readyCh == nil {
			wailsContext.readyCh = make(chan struct{})
		}
		readyCh = wailsContext.readyCh
		wailsContext.mu.Unlock()
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-readyCh:
		return ensureWailsContext()
	case <-timer.C:
		return nil, false
	}
}
