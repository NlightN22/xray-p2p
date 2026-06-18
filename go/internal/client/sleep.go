package client

import (
	"context"
	"errors"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/xrayguard"
)

const loopProtectionQuarantineDelay = 3 * time.Minute

func sleepWithContext(ctx context.Context, d time.Duration) bool {
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

func loopProtectionEvent(err error) (xrayguard.Event, bool) {
	var event xrayguard.Event
	if errors.As(err, &event) {
		return event, true
	}
	return xrayguard.Event{}, false
}
