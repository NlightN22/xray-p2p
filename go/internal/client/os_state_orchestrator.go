package client

import (
	"context"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

type OSStatePhase string

const (
	OSStatePhaseDisabled     OSStatePhase = "disabled"
	OSStatePhaseSplit        OSStatePhase = "split"
	OSStatePhaseFullPending  OSStatePhase = "full_pending"
	OSStatePhaseFullApplied  OSStatePhase = "full_applied"
	OSStatePhaseErrorLatched OSStatePhase = "error_latched"
)

type OSMode string

const (
	OSModeDisabled OSMode = "disabled"
	OSModeSplit    OSMode = "split"
	OSModeFull     OSMode = "full"
)

type DesiredOSState struct {
	TunEnabled bool
	TunName    string
	TunAddr    string
	TunMTU     int
	TunMode    string
	DNSServers []string

	FullTunnelVerbose bool
	FullTunnelTag     string

	Install clientInstallState
}

func (d DesiredOSState) Mode() OSMode {
	if !d.TunEnabled {
		return OSModeDisabled
	}
	if strings.EqualFold(strings.TrimSpace(d.TunMode), "full") {
		return OSModeFull
	}
	return OSModeSplit
}

type ObservedOSState struct {
	TunReady bool

	TunIfIndex    int
	TunIPv4       string
	TunOperStatus string
	TunDadState   string
}

type RollbackReason string

const (
	RollbackReasonServiceStop RollbackReason = "service_stop"
	RollbackReasonModeSwitch  RollbackReason = "mode_switch"
	RollbackReasonApplyFailed RollbackReason = "apply_failed"
)

type OSStatePlan struct {
	Desired  DesiredOSState
	Observed ObservedOSState

	WasFullEnabled bool

	RollbackFull bool
	EnsureFull   bool
	EnsureSplit  bool
	RemoveSplit  bool

	TargetPhase OSStatePhase
}

type OSStateResult struct {
	Phase          OSStatePhase
	Observed       ObservedOSState
	RedirectApplied bool
	RedirectCount   int
	FullApplied     bool
	FullBypassCount int
}

type OSStateOrchestrator interface {
	Reconcile(ctx context.Context, desired DesiredOSState, reason ReconcileReason) (OSStateResult, error)
	Rollback(ctx context.Context, reason RollbackReason, desired DesiredOSState) error
}

type osStateDriver interface {
	EnsureTunReady(ctx context.Context, desired DesiredOSState) (ObservedOSState, error)
	EnsureSplit(ctx context.Context, desired DesiredOSState) (bool, error)
	RemoveSplit(ctx context.Context, desired DesiredOSState) error
	EnsureFull(ctx context.Context, desired DesiredOSState) (bool, error)
	RollbackFull(ctx context.Context, desired DesiredOSState) error
}

type osStateOrchestrator struct {
	paths  clientPaths
	driver osStateDriver
}

func NewOSStateOrchestrator(paths clientPaths, driver osStateDriver) OSStateOrchestrator {
	return &osStateOrchestrator{
		paths:  paths,
		driver: driver,
	}
}

func (o *osStateOrchestrator) plan(desired DesiredOSState, observed ObservedOSState) (OSStatePlan, error) {
	fullState, err := loadFullTunnelState(o.paths.fullState)
	if err != nil {
		return OSStatePlan{}, err
	}

	desiredMode := desired.Mode()
	plan := OSStatePlan{
		Desired:        desired,
		Observed:       observed,
		WasFullEnabled: fullState.Enabled,
	}

	plan.RollbackFull = fullState.Enabled && desiredMode != OSModeFull
	plan.RemoveSplit = desiredMode == OSModeDisabled
	plan.EnsureFull = desiredMode == OSModeFull && observed.TunReady
	plan.EnsureSplit = desiredMode != OSModeDisabled && desired.TunEnabled && observed.TunReady

	switch desiredMode {
	case OSModeFull:
		if observed.TunReady {
			plan.TargetPhase = OSStatePhaseFullApplied
		} else {
			plan.TargetPhase = OSStatePhaseFullPending
		}
	case OSModeSplit:
		plan.TargetPhase = OSStatePhaseSplit
	default:
		plan.TargetPhase = OSStatePhaseDisabled
	}

	return plan, nil
}

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

func (o *osStateOrchestrator) recordPending(reason string, err error) (*PendingRetryError, error) {
	now := time.Now().UTC()
	state, loadErr := loadFullTunnelState(o.paths.fullState)
	if loadErr != nil {
		return nil, loadErr
	}

	if state.Phase != string(OSStatePhaseFullPending) || state.PendingSince.IsZero() {
		state.PendingSince = now
		state.RetryCount = 0
	} else {
		state.RetryCount++
	}
	delay := computeRetryBackoff(state.RetryCount)
	state.NextRetryAt = now.Add(delay)
	state.PendingReason = strings.TrimSpace(reason)
	state.Phase = string(OSStatePhaseFullPending)
	state.LastError = errString(err)
	state.LastErrorAt = nowOrZero(err)

	if saveErr := saveFullTunnelState(o.paths.fullState, state); saveErr != nil {
		return nil, saveErr
	}
	return &PendingRetryError{Reason: reason, Err: err, Delay: delay}, nil
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

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func nowOrZero(err error) time.Time {
	if err == nil {
		return time.Time{}
	}
	return time.Now().UTC()
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
