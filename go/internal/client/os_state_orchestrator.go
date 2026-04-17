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

type ApplyStage string

const (
	ApplyStagePreStart  ApplyStage = "pre_start"
	ApplyStagePostReady ApplyStage = "post_ready"
)

type RollbackReason string

const (
	RollbackReasonServiceStop RollbackReason = "service_stop"
	RollbackReasonModeSwitch  RollbackReason = "mode_switch"
	RollbackReasonApplyFailed RollbackReason = "apply_failed"
)

type OSStatePlan struct {
	Stage   ApplyStage
	Desired DesiredOSState
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
	RedirectApplied bool
	RedirectCount   int
	FullApplied     bool
	FullBypassCount int
}

type OSStateOrchestrator interface {
	Plan(stage ApplyStage, desired DesiredOSState, observed ObservedOSState) (OSStatePlan, error)
	Apply(ctx context.Context, plan OSStatePlan) (OSStateResult, error)
	Rollback(ctx context.Context, reason RollbackReason, desired DesiredOSState) error
	OnServiceStart(ctx context.Context) error
	OnServiceStop(ctx context.Context, desired DesiredOSState) error
	OnXrayRestart(ctx context.Context, desired DesiredOSState) error
	MarkFullPending(err error) error
}

type osStateDriver interface {
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

func (o *osStateOrchestrator) OnServiceStart(_ context.Context) error {
	return nil
}

func (o *osStateOrchestrator) OnServiceStop(ctx context.Context, desired DesiredOSState) error {
	return o.Rollback(ctx, RollbackReasonServiceStop, desired)
}

func (o *osStateOrchestrator) OnXrayRestart(_ context.Context, _ DesiredOSState) error {
	return nil
}

func (o *osStateOrchestrator) MarkFullPending(err error) error {
	state, loadErr := loadFullTunnelState(o.paths.fullState)
	if loadErr != nil {
		return loadErr
	}
	state.Phase = string(OSStatePhaseFullPending)
	state.LastError = errString(err)
	state.LastErrorAt = nowOrZero(err)
	return saveFullTunnelState(o.paths.fullState, state)
}

func (o *osStateOrchestrator) Plan(stage ApplyStage, desired DesiredOSState, observed ObservedOSState) (OSStatePlan, error) {
	fullState, err := loadFullTunnelState(o.paths.fullState)
	if err != nil {
		return OSStatePlan{}, err
	}

	desiredMode := desired.Mode()
	plan := OSStatePlan{
		Stage:          stage,
		Desired:        desired,
		Observed:       observed,
		WasFullEnabled: fullState.Enabled,
	}

	switch stage {
	case ApplyStagePreStart:
		plan.RollbackFull = fullState.Enabled && desiredMode != OSModeFull
		plan.RemoveSplit = desiredMode == OSModeDisabled
	case ApplyStagePostReady:
		if desiredMode == OSModeFull {
			plan.EnsureFull = true
		}
		if desiredMode == OSModeSplit {
			plan.EnsureSplit = true
		}
	}

	switch desiredMode {
	case OSModeFull:
		if stage == ApplyStagePreStart {
			plan.TargetPhase = OSStatePhaseFullPending
		} else {
			plan.TargetPhase = OSStatePhaseFullApplied
		}
	case OSModeSplit:
		plan.TargetPhase = OSStatePhaseSplit
	default:
		plan.TargetPhase = OSStatePhaseDisabled
	}

	return plan, nil
}

func (o *osStateOrchestrator) Apply(ctx context.Context, plan OSStatePlan) (OSStateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	result := OSStateResult{
		Phase: plan.TargetPhase,
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
	state.Phase = string(phase)
	if phase != OSStatePhaseFullPending {
		state.LastError = ""
		state.LastErrorAt = time.Time{}
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
