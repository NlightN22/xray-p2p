package client

import (
	"strings"
	"time"
)

func (o *osStateOrchestrator) recordPending(reason string, err error) (*PendingRetryError, error) {
	now := time.Now().UTC()
	state, loadErr := loadFullTunnelState(o.paths.fullState)
	if loadErr != nil {
		return nil, loadErr
	}

	if state.Phase != string(OSStatePhaseFullPending) || state.PendingSince.IsZero() {
		state.PendingSince = now
		state.RetryCount = 0
	} else {
		state.RetryCount++
	}
	delay := computeRetryBackoff(state.RetryCount)
	state.NextRetryAt = now.Add(delay)
	state.PendingReason = strings.TrimSpace(reason)
	state.Phase = string(OSStatePhaseFullPending)
	state.LastError = errString(err)
	state.LastErrorAt = nowOrZero(err)

	if saveErr := saveFullTunnelState(o.paths.fullState, state); saveErr != nil {
		return nil, saveErr
	}
	return &PendingRetryError{Reason: reason, Err: err, Delay: delay}, nil
}
