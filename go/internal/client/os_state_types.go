package client

import (
	"context"
	"strings"
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
	Phase           OSStatePhase
	Observed        ObservedOSState
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
