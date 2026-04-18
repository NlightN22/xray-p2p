package client

import (
	"context"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func (o *osStateOrchestrator) apply(ctx context.Context, plan OSStatePlan) (OSStateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	result := OSStateResult{
		Phase:    plan.TargetPhase,
		Observed: plan.Observed,
	}

	if plan.RollbackFull {
		if err := o.driver.RollbackFull(ctx, plan.Desired); err != nil {
			return result, err
		}
	}
	if plan.RemoveSplit {
		if err := o.driver.RemoveSplit(ctx, plan.Desired); err != nil {
			return result, err
		}
	}
	if plan.EnsureSplit {
		applied, err := o.driver.EnsureSplit(ctx, plan.Desired)
		if err != nil {
			logging.Warn("split routes apply failed", "err", err)
		}
		result.RedirectApplied = applied
		result.RedirectCount = countRedirectCIDRs(plan.Desired.Install.Redirects)
	}
	if plan.EnsureFull {
		applied, err := o.driver.EnsureFull(ctx, plan.Desired)
		if err != nil {
			if markErr := o.latchError(OSStatePhaseErrorLatched, err); markErr != nil {
				return result, markErr
			}
			result.Phase = OSStatePhaseErrorLatched
			if !plan.WasFullEnabled {
				if rbErr := o.driver.RollbackFull(ctx, plan.Desired); rbErr != nil {
					logging.Warn("full-tunnel rollback failed after apply failure", "err", rbErr)
				}
			}
			logging.Warn("full-tunnel apply failed", "err", err)
			return result, nil
		}
		result.FullApplied = applied
		state, loadErr := loadFullTunnelState(o.paths.fullState)
		if loadErr == nil {
			result.FullBypassCount = len(state.BypassRoutes)
		}
	}

	if err := o.setPhase(plan.TargetPhase); err != nil {
		return result, err
	}
	return result, nil
}

func (o *osStateOrchestrator) setPhase(phase OSStatePhase) error {
	state, err := loadFullTunnelState(o.paths.fullState)
	if err != nil {
		return err
	}
	prevPhase := strings.TrimSpace(state.Phase)
	state.Phase = string(phase)
	if phase != OSStatePhaseFullPending {
		state.LastError = ""
		state.LastErrorAt = time.Time{}
		state.PendingReason = ""
		state.PendingSince = time.Time{}
		state.RetryCount = 0
		state.NextRetryAt = time.Time{}
	}
	if strings.EqualFold(prevPhase, string(OSStatePhaseFullPending)) && phase == OSStatePhaseFullApplied {
		logging.Info("full-tunnel pending resolved")
	}
	return saveFullTunnelState(o.paths.fullState, state)
}

func (o *osStateOrchestrator) latchError(phase OSStatePhase, err error) error {
	state, loadErr := loadFullTunnelState(o.paths.fullState)
	if loadErr != nil {
		return loadErr
	}
	state.Phase = string(phase)
	state.LastError = errString(err)
	state.LastErrorAt = nowOrZero(err)
	state.PendingReason = ""
	state.PendingSince = time.Time{}
	state.RetryCount = 0
	state.NextRetryAt = time.Time{}
	return saveFullTunnelState(o.paths.fullState, state)
}

func countRedirectCIDRs(rules []redirect.Rule) int {
	if len(rules) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(rules))
	count := 0
	for _, rule := range rules {
		if rule.Kind() != redirect.KindCIDR {
			continue
		}
		if rule.NoRoutes {
			continue
		}
		value := strings.TrimSpace(rule.Value())
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		count++
	}
	return count
}
