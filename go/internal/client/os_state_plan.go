package client

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
