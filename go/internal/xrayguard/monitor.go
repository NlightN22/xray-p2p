package xrayguard

import (
	"context"
	"errors"
	"time"
)

func Monitor(ctx context.Context, pid int, opts Options) <-chan Event {
	opts = normalizeOptions(opts)
	events := make(chan Event, 1)
	if pid <= 0 || opts.Collector == nil {
		close(events)
		return events
	}

	go func() {
		defer close(events)

		detector := NewDetector(opts)
		ticker := time.NewTicker(opts.Interval)
		defer ticker.Stop()

		for {
			sample, err := opts.Collector.Sample(ctx, pid)
			if err == nil {
				if event, ok := detector.Observe(pid, sample); ok {
					events <- event
					return
				}
			} else if errors.Is(err, ErrUnsupported) {
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return events
}
