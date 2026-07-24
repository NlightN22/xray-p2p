package client

import (
	"errors"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
)

func TestXP2PDiagCheckDoesNotCallHeartbeatReport(t *testing.T) {
	calls := 0
	err := completeHeartbeatCheck(heartbeat.CapabilityXP2PDiag, func() error {
		calls++
		return errors.New("closed heartbeat endpoint")
	})
	if err != nil {
		t.Fatalf("ping-only check returned report error: %v", err)
	}
	if calls != 0 {
		t.Fatalf("heartbeat report was called %d times", calls)
	}
}

func TestFullHeartbeatCheckRequiresSuccessfulReport(t *testing.T) {
	want := errors.New("report failed")
	calls := 0
	err := completeHeartbeatCheck(heartbeat.CapabilityXP2PHeartbeat, func() error {
		calls++
		return want
	})
	if !errors.Is(err, want) || calls != 1 {
		t.Fatalf("full check did not require report: calls=%d err=%v", calls, err)
	}
}
