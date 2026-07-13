package client

import (
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type heartbeatBackoff struct {
	failures  int
	skipUntil time.Time
}

func (r *heartbeatRunner) endpointInBackoff(endpoint clientEndpointRecord, now time.Time) bool {
	if r == nil || len(r.backoff) == 0 {
		return false
	}
	state, ok := r.backoff[heartbeatBackoffKey(endpoint)]
	if !ok || state.skipUntil.IsZero() || !now.Before(state.skipUntil) {
		return false
	}
	logging.Debug("client heartbeat skipped during endpoint backoff",
		"host", endpoint.Hostname,
		"tag", endpoint.Tag,
		"until", state.skipUntil.UTC().Format(time.RFC3339),
		"failures", state.failures,
	)
	return true
}

func (r *heartbeatRunner) recordHeartbeatFailure(endpoint clientEndpointRecord) {
	if r.backoff == nil {
		r.backoff = map[string]heartbeatBackoff{}
	}
	key := heartbeatBackoffKey(endpoint)
	state := r.backoff[key]
	state.failures++
	delay := r.heartbeatBackoffDelay(state.failures)
	state.skipUntil = time.Now().Add(delay)
	r.backoff[key] = state
	logging.Debug("client heartbeat endpoint backoff updated",
		"host", endpoint.Hostname,
		"tag", endpoint.Tag,
		"delay", delay.String(),
		"failures", state.failures,
	)
}

func (r *heartbeatRunner) recordHeartbeatSuccess(endpoint clientEndpointRecord) {
	if r.backoff == nil {
		return
	}
	delete(r.backoff, heartbeatBackoffKey(endpoint))
}

func (r *heartbeatRunner) heartbeatBackoffDelay(failures int) time.Duration {
	base := r.interval
	if base <= 0 {
		base = 2 * time.Second
	}
	if r.timeout > base {
		base = r.timeout
	}
	if failures <= 1 {
		return base
	}
	multiplier := 1 << min(failures-1, 4)
	delay := time.Duration(multiplier) * base
	maxDelay := 30 * time.Second
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

func heartbeatBackoffKey(endpoint clientEndpointRecord) string {
	tag := strings.ToLower(strings.TrimSpace(endpoint.Tag))
	user := strings.ToLower(strings.TrimSpace(endpoint.User))
	if user == "" {
		return tag
	}
	return tag + "|" + user
}
