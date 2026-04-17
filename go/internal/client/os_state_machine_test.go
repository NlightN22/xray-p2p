package client

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

type fakeOSStateDriver struct {
	ensureTunReadyCalls int
	ensureSplitCalls    int
	removeSplitCalls    int
	ensureFullCalls     int
	rollbackFullCalls   int

	observed ObservedOSState
	tunErr   error

	ensureFullErr error
}

func (d *fakeOSStateDriver) EnsureTunReady(_ context.Context, _ DesiredOSState) (ObservedOSState, error) {
	d.ensureTunReadyCalls++
	if d.tunErr != nil {
		return ObservedOSState{}, d.tunErr
	}
	return d.observed, nil
}

func (d *fakeOSStateDriver) EnsureSplit(_ context.Context, _ DesiredOSState) (bool, error) {
	d.ensureSplitCalls++
	return true, nil
}

func (d *fakeOSStateDriver) RemoveSplit(_ context.Context, _ DesiredOSState) error {
	d.removeSplitCalls++
	return nil
}

func (d *fakeOSStateDriver) EnsureFull(_ context.Context, _ DesiredOSState) (bool, error) {
	d.ensureFullCalls++
	if d.ensureFullErr != nil {
		return false, d.ensureFullErr
	}
	return true, nil
}

func (d *fakeOSStateDriver) RollbackFull(_ context.Context, _ DesiredOSState) error {
	d.rollbackFullCalls++
	return nil
}

func TestOSStateReconcileFullPendingRecordsRetry(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "tun-full.json")
	if err := saveFullTunnelState(statePath, fullTunnelState{Enabled: false}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	driver := &fakeOSStateDriver{
		tunErr: &OSPendingError{Reason: "adapter_not_ready", Err: errors.New("not ready")},
	}
	orch := NewOSStateOrchestrator(clientPaths{fullState: statePath}, driver)
	desired := DesiredOSState{TunEnabled: true, TunMode: "full"}

	start := time.Now().UTC()
	_, err := orch.Reconcile(context.Background(), desired, ReconcileReasonServiceStart)
	if err == nil {
		t.Fatalf("expected pending retry error")
	}
	var pending *PendingRetryError
	if !errors.As(err, &pending) {
		t.Fatalf("expected PendingRetryError, got %T: %v", err, err)
	}
	if pending.Delay <= 0 {
		t.Fatalf("expected positive backoff, got %v", pending.Delay)
	}

	state, err := loadFullTunnelState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Phase != string(OSStatePhaseFullPending) {
		t.Fatalf("expected phase %q, got %q", OSStatePhaseFullPending, state.Phase)
	}
	if state.PendingReason != "adapter_not_ready" {
		t.Fatalf("expected pending_reason adapter_not_ready, got %q", state.PendingReason)
	}
	if state.NextRetryAt.Before(start) {
		t.Fatalf("expected next_retry_at to be set")
	}
}

func TestOSStateReconcileRollsBackOnModeSwitch(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "tun-full.json")
	if err := saveFullTunnelState(statePath, fullTunnelState{Enabled: true}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	driver := &fakeOSStateDriver{observed: ObservedOSState{TunReady: true}}
	orch := NewOSStateOrchestrator(clientPaths{fullState: statePath}, driver)
	desired := DesiredOSState{TunEnabled: true, TunMode: "split"}

	if _, err := orch.Reconcile(context.Background(), desired, ReconcileReasonModeSwitch); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if driver.rollbackFullCalls != 1 {
		t.Fatalf("expected rollbackFullCalls=1, got %d", driver.rollbackFullCalls)
	}
}

func TestOSStateReconcileFullFailureLatchesError(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "tun-full.json")
	if err := saveFullTunnelState(statePath, fullTunnelState{Enabled: false}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	driver := &fakeOSStateDriver{
		observed:      ObservedOSState{TunReady: true},
		ensureFullErr: errors.New("apply failed"),
	}
	orch := NewOSStateOrchestrator(clientPaths{fullState: statePath}, driver)
	desired := DesiredOSState{TunEnabled: true, TunMode: "full"}

	if _, err := orch.Reconcile(context.Background(), desired, ReconcileReasonServiceStart); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	state, err := loadFullTunnelState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Phase != string(OSStatePhaseErrorLatched) {
		t.Fatalf("expected phase %q, got %q", OSStatePhaseErrorLatched, state.Phase)
	}
	if state.LastError == "" {
		t.Fatalf("expected last_error to be set")
	}
	if driver.rollbackFullCalls != 1 {
		t.Fatalf("expected rollbackFullCalls=1 for initial enable failure, got %d", driver.rollbackFullCalls)
	}
}

func TestOSStateReconcileFullFailureDoesNotRollbackWhenAlreadyEnabled(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "tun-full.json")
	if err := saveFullTunnelState(statePath, fullTunnelState{Enabled: true}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	driver := &fakeOSStateDriver{
		observed:      ObservedOSState{TunReady: true},
		ensureFullErr: errors.New("apply failed"),
	}
	orch := NewOSStateOrchestrator(clientPaths{fullState: statePath}, driver)
	desired := DesiredOSState{TunEnabled: true, TunMode: "full"}

	if _, err := orch.Reconcile(context.Background(), desired, ReconcileReasonServiceRestart); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if driver.rollbackFullCalls != 0 {
		t.Fatalf("expected rollbackFullCalls=0 when already enabled, got %d", driver.rollbackFullCalls)
	}
}

