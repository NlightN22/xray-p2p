//go:build linux || windows

package client

import (
	"errors"
	"testing"
)

func TestRetryClientDesiredMutationReloadsAfterConflict(t *testing.T) {
	attempts := 0
	err := retryClientDesiredMutation(func() error {
		attempts++
		if attempts < 3 {
			return errClientDesiredConflict
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry mutation: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetryClientDesiredMutationDoesNotRetryOtherErrors(t *testing.T) {
	want := errors.New("invalid redirect")
	attempts := 0
	err := retryClientDesiredMutation(func() error {
		attempts++
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
