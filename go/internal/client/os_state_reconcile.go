package client

import (
	"context"
	"strings"
)

type ReconcileReason string

const (
	ReconcileReasonServiceStart   ReconcileReason = "service_start"
	ReconcileReasonServiceRestart ReconcileReason = "service_restart"
	ReconcileReasonApplyRequest   ReconcileReason = "apply_request"
	ReconcileReasonModeSwitch     ReconcileReason = "mode_switch"
)

func (o *osStateOrchestrator) Reconcile(ctx context.Context, desired DesiredOSState, reason ReconcileReason) (OSStateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	observed, obsErr := o.driver.EnsureTunReady(ctx, desired)
	if obsErr != nil {
		if desired.Mode() == OSModeFull {
			if pendingReason, ok := isPendingErr(obsErr); ok {
				pending, recordErr := o.recordPending(pendingReason, obsErr)
				if recordErr != nil {
					return OSStateResult{Phase: OSStatePhaseErrorLatched, Observed: observed}, recordErr
				}
				logPendingRetry(pending.Reason, pending.Err, pending.Delay, reason)
				return OSStateResult{Phase: OSStatePhaseFullPending, Observed: observed}, pending
			}
		}
		return OSStateResult{Phase: OSStatePhaseErrorLatched, Observed: observed}, obsErr
	}

	plan, err := o.plan(desired, observed)
	if err != nil {
		return OSStateResult{Phase: OSStatePhaseErrorLatched, Observed: observed}, err
	}
	return o.apply(ctx, plan)
}

func (o *osStateOrchestrator) Rollback(ctx context.Context, reason RollbackReason, desired DesiredOSState) error {
	if ctx == nil {
		ctx = context.Background()
	}

	fullState, err := loadFullTunnelState(o.paths.fullState)
	if err != nil {
		return err
	}
	if fullState.Enabled {
		if err := o.driver.RollbackFull(ctx, desired); err != nil {
			return err
		}
	}
	if strings.EqualFold(string(reason), string(RollbackReasonServiceStop)) || desired.Mode() == OSModeDisabled {
		if err := o.driver.RemoveSplit(ctx, desired); err != nil {
			return err
		}
	}

	target := OSStatePhaseDisabled
	if desired.Mode() == OSModeSplit {
		target = OSStatePhaseSplit
	}
	return o.setPhase(target)
}
