//go:build windows || linux

package server

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
)

type schedulerFetcher func(context.Context, identitysync.ProviderRef) (identitysync.Snapshot, error)

func (f schedulerFetcher) FetchSnapshot(ctx context.Context, provider identitysync.ProviderRef) (identitysync.Snapshot, error) {
	return f(ctx, provider)
}

func TestIdentitySchedulerStopCancelsAndJoinsActiveSync(t *testing.T) {
	previous := IdentitySnapshotFetcher
	t.Cleanup(func() { IdentitySnapshotFetcher = previous })
	started := make(chan struct{})
	exited := make(chan struct{})
	IdentitySnapshotFetcher = schedulerFetcher(func(ctx context.Context, _ identitysync.ProviderRef) (identitysync.Snapshot, error) {
		close(started)
		<-ctx.Done()
		close(exited)
		return identitysync.Snapshot{}, ctx.Err()
	})

	stop := startIdentitySyncScheduler(context.Background(), schedulerConfig())
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("identity sync did not start")
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := stop(stopCtx); err != nil {
		t.Fatalf("stop scheduler: %v", err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("stop returned before active sync exited")
	}
}

func TestIdentitySchedulerStopReportsDeadline(t *testing.T) {
	previous := IdentitySnapshotFetcher
	t.Cleanup(func() { IdentitySnapshotFetcher = previous })
	started := make(chan struct{})
	release := make(chan struct{})
	IdentitySnapshotFetcher = schedulerFetcher(func(context.Context, identitysync.ProviderRef) (identitysync.Snapshot, error) {
		close(started)
		<-release
		return identitysync.Snapshot{}, errors.New("released")
	})

	stop := startIdentitySyncScheduler(context.Background(), schedulerConfig())
	<-started
	stopCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	err := stop(stopCtx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stop error = %v, want deadline exceeded", err)
	}
	close(release)
	joinCtx, joinCancel := context.WithTimeout(context.Background(), time.Second)
	defer joinCancel()
	if err := stop(joinCtx); err != nil {
		t.Fatalf("join released scheduler: %v", err)
	}
}

func TestIdentitySchedulerConcurrentStopCallsCancelAndJoinOnce(t *testing.T) {
	previous := IdentitySnapshotFetcher
	t.Cleanup(func() { IdentitySnapshotFetcher = previous })
	started := make(chan struct{})
	release := make(chan struct{})
	var fetches atomic.Int32
	IdentitySnapshotFetcher = schedulerFetcher(func(ctx context.Context, _ identitysync.ProviderRef) (identitysync.Snapshot, error) {
		if fetches.Add(1) == 1 {
			close(started)
		}
		<-ctx.Done()
		<-release
		return identitysync.Snapshot{}, ctx.Err()
	})

	stop := startIdentitySyncScheduler(context.Background(), schedulerConfig())
	<-started

	const callers = 16
	ready := sync.WaitGroup{}
	ready.Add(callers)
	begin := make(chan struct{})
	results := make(chan error, callers)
	for range callers {
		go func() {
			ready.Done()
			<-begin
			stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			results <- stop(stopCtx)
		}()
	}
	ready.Wait()
	close(begin)
	select {
	case err := <-results:
		t.Fatalf("concurrent stop returned before scheduler worker exited: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	for range callers {
		if err := <-results; err != nil {
			t.Fatalf("concurrent stop error: %v", err)
		}
	}
	if got := fetches.Load(); got != 1 {
		t.Fatalf("identity sync fetches = %d, want 1", got)
	}
}

func schedulerConfig() config.Config {
	var cfg config.Config
	cfg.Server.IdentityProvider.InstanceID = "test"
	cfg.Server.IdentityProvider.Kind = string(identitysync.ProviderSCIM)
	cfg.Server.IdentityProvider.Interval = time.Hour.String()
	return cfg
}
