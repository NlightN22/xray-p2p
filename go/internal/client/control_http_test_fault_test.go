package client

import (
	"context"
	"testing"
	"time"
)

func TestControlTransportLeakFaultRequiresTestMode(t *testing.T) {
	t.Setenv("XP2P_TEST_MODE", "1")
	t.Setenv("XP2P_TEST_CONTROL_TRANSPORT_LEAK", "1")
	pool := newControlHTTPPool(time.Second, "")
	endpoint := clientEndpointRecord{Hostname: "example.test", AllowInsecure: true}
	first := pool.client(endpoint)
	second := pool.client(endpoint)
	if err := pool.prune(context.Background(), []clientEndpointRecord{endpoint}); err != nil {
		t.Fatal(err)
	}
	if first == second || len(pool.clients) != 2 {
		t.Fatalf("fault did not retain request-scoped clients: count=%d", len(pool.clients))
	}
}

func TestSubscriptionIntervalFaultRequiresTestMode(t *testing.T) {
	t.Setenv("XP2P_TEST_MODE", "1")
	t.Setenv("XP2P_TEST_SUBSCRIPTION_INTERVAL", "25ms")
	if got := testSubscriptionSyncInterval(time.Minute); got != 25*time.Millisecond {
		t.Fatalf("interval = %s, want 25ms", got)
	}
}
