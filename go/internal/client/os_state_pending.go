package client

import (
	"errors"
	"fmt"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type OSPendingError struct {
	Reason string
	Err    error
}

func (e *OSPendingError) Error() string {
	if e == nil {
		return ""
	}
	reason := e.Reason
	if reason == "" {
		reason = "pending"
	}
	if e.Err == nil {
		return reason
	}
	return fmt.Sprintf("%s: %v", reason, e.Err)
}

func (e *OSPendingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type PendingRetryError struct {
	Reason string
	Err    error
	Delay  time.Duration
}

func (e *PendingRetryError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return "pending retry"
	}
	return fmt.Sprintf("pending retry: %v", e.Err)
}

func (e *PendingRetryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func isPendingErr(err error) (string, bool) {
	var pending *OSPendingError
	if errors.As(err, &pending) {
		return pending.Reason, true
	}
	return "", false
}

func computeRetryBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > 6 {
		attempt = 6
	}
	delay := 2 * time.Second
	delay <<= attempt
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return delay
}

func logPendingRetry(reason string, err error, delay time.Duration, trigger ReconcileReason) {
	msg := "full-tunnel pending; deferring route apply until restart"
	fields := []any{"reason", reason, "delay", delay.String(), "trigger", string(trigger)}
	if err != nil {
		fields = append(fields, "err", err.Error())
	}
	logging.Info(msg, fields...)
}

