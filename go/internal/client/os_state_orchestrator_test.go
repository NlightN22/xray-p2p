package client

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

type fakeOSStateDriver struct {
	ensureSplitCalls  int
	removeSplitCalls  int
	ensureFullCalls   int
	rollbackFullCalls int

	ensureFullErr error
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

func TestOSStatePlanPreStartFullPending(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "tun-full.json")
	if err := saveFullTunnelState(statePath, fullTunnelState{Enabled: false}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	driver := &fakeOSStateDriver{}
	orch := NewOSStateOrchestrator(clientPaths{fullState: statePath}, driver)
	desired := DesiredOSState{TunEnabled: true, TunMode: "full"}

	plan, err := orch.Plan(ApplyStagePreStart, desired, ObservedOSState{})
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if plan.RollbackFull {
		t.Fatalf("expected RollbackFull=false for desired full")
	}
	if plan.TargetPhase != OSStatePhaseFullPending {
		t.Fatalf("expected phase %q, got %q", OSStatePhaseFullPending, plan.TargetPhase)
	}

	if _, err := orch.Apply(context.Background(), plan); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	state, err := loadFullTunnelState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Phase != string(OSStatePhaseFullPending) {
		t.Fatalf("expected state phase %q, got %q", OSStatePhaseFullPending, state.Phase)
	}
}

func TestOSStatePlanPreStartRollsBackOnModeSwitch(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "tun-full.json")
	if err := saveFullTunnelState(statePath, fullTunnelState{Enabled: true}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	driver := &fakeOSStateDriver{}
	orch := NewOSStateOrchestrator(clientPaths{fullState: statePath}, driver)
	desired := DesiredOSState{TunEnabled: true, TunMode: "split"}

	plan, err := orch.Plan(ApplyStagePreStart, desired, ObservedOSState{})
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if !plan.RollbackFull {
		t.Fatalf("expected RollbackFull=true on full->split switch")
	}
	if _, err := orch.Apply(context.Background(), plan); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if driver.rollbackFullCalls != 1 {
		t.Fatalf("expected rollbackFullCalls=1, got %d", driver.rollbackFullCalls)
	}
}

func TestOSStateApplyFullFailureLatchesError(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "tun-full.json")
	if err := saveFullTunnelState(statePath, fullTunnelState{Enabled: false}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	applyErr := errors.New("apply failed")
	driver := &fakeOSStateDriver{ensureFullErr: applyErr}
	orch := NewOSStateOrchestrator(clientPaths{fullState: statePath}, driver)
	desired := DesiredOSState{TunEnabled: true, TunMode: "full"}

	plan, err := orch.Plan(ApplyStagePostReady, desired, ObservedOSState{TunReady: true})
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if _, err := orch.Apply(context.Background(), plan); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	state, err := loadFullTunnelState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Phase != string(OSStatePhaseErrorLatched) {
		t.Fatalf("expected phase %q, got %q", OSStatePhaseErrorLatched, state.Phase)
	}
	if state.LastError == "" {
		t.Fatalf("expected LastError to be set")
	}
	if driver.rollbackFullCalls != 1 {
		t.Fatalf("expected rollbackFullCalls=1 for initial enable failure, got %d", driver.rollbackFullCalls)
	}
}

func TestOSStateApplyFullFailureDoesNotRollbackWhenAlreadyEnabled(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "tun-full.json")
	if err := saveFullTunnelState(statePath, fullTunnelState{Enabled: true}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	driver := &fakeOSStateDriver{ensureFullErr: errors.New("apply failed")}
	orch := NewOSStateOrchestrator(clientPaths{fullState: statePath}, driver)
	desired := DesiredOSState{TunEnabled: true, TunMode: "full"}

	plan, err := orch.Plan(ApplyStagePostReady, desired, ObservedOSState{TunReady: true})
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if _, err := orch.Apply(context.Background(), plan); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if driver.rollbackFullCalls != 0 {
		t.Fatalf("expected rollbackFullCalls=0 when already enabled, got %d", driver.rollbackFullCalls)
	}
}

