package clientcmd

import (
	"testing"
	"time"
)

func TestTestHeartbeatIntervalRequiresExplicitTestMode(t *testing.T) {
	t.Setenv("XP2P_TEST_HEARTBEAT_INTERVAL", "250ms")
	got, err := testHeartbeatInterval(2 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got != 2*time.Second {
		t.Fatalf("interval = %s, want production fallback", got)
	}
}

func TestTestHeartbeatIntervalAppliesPositiveDuration(t *testing.T) {
	t.Setenv("XP2P_TEST_MODE", "1")
	t.Setenv("XP2P_TEST_HEARTBEAT_INTERVAL", "250ms")
	got, err := testHeartbeatInterval(2 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got != 250*time.Millisecond {
		t.Fatalf("interval = %s, want 250ms", got)
	}
}

func TestTestHeartbeatIntervalRejectsInvalidValue(t *testing.T) {
	t.Setenv("XP2P_TEST_MODE", "1")
	t.Setenv("XP2P_TEST_HEARTBEAT_INTERVAL", "zero")
	if _, err := testHeartbeatInterval(2 * time.Second); err == nil {
		t.Fatal("expected invalid test heartbeat interval to fail")
	}
}
