package xraylive

import (
	"context"
	"errors"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/runtimeapply"
)

func applyMixedRuntimeDiff(ctx context.Context, opts Options, req apply.Request, role, address string, artifacts Artifacts, diff runtimeapply.Diff) (RuntimeApplyResult, error) {
	appliers, closeAll, err := openMixedAppliers(ctx, opts, address, diff)
	if err != nil {
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	defer closeAll()

	applied := make([]runtimeapply.Diff, 0, 4)
	for _, phase := range mixedApplyPhases(diff) {
		if isEmptyDiff(phase) {
			continue
		}
		if err := applyMixedPhase(ctx, appliers, phase); err != nil {
			rollbackErr := rollbackMixedPhases(ctx, appliers, applied)
			if rollbackErr != nil {
				logging.Warn("runtime mixed apply rollback failed", "role", role, "request_id", req.ID, "err", rollbackErr)
			}
			writeRuntimeApplyError(opts, req, err)
			return RuntimeApplyFailed, nil
		}
		applied = append(applied, phase)
	}
	if runtimeVerificationFailed(ctx, opts, req, func() error {
		return rollbackMixedPhases(ctx, appliers, applied)
	}) {
		return RuntimeApplyFailed, nil
	}

	if err := publishLiveArtifacts(opts, artifacts); err != nil {
		rollbackErr := rollbackMixedPhases(ctx, appliers, applied)
		reason := err
		if rollbackErr != nil {
			reason = fmt.Errorf("%w; runtime rollback failed: %v", err, rollbackErr)
		}
		writeRuntimeApplyError(opts, req, reason)
		return RuntimeApplyFailed, nil
	}
	if err := commitDesiredOrRestoreLive(opts); err != nil {
		rollbackErr := rollbackMixedPhases(ctx, appliers, applied)
		reason := err
		if rollbackErr != nil {
			reason = fmt.Errorf("%w; runtime rollback failed: %v", err, rollbackErr)
		}
		writeRuntimeApplyError(opts, req, reason)
		return RuntimeApplyFailed, nil
	}

	cleanupRuntimeApplyMarkers(opts, req)
	logging.Info("runtime mixed apply completed", "role", role, "request_id", req.ID)
	return RuntimeApplyApplied, nil
}

type mixedAppliers struct {
	routing     runtimeapply.RoutingApplier
	outbound    runtimeapply.OutboundApplier
	inboundUser runtimeapply.InboundUserApplier
}

func openMixedAppliers(ctx context.Context, opts Options, address string, diff runtimeapply.Diff) (mixedAppliers, func(), error) {
	var result mixedAppliers
	closers := make([]func() error, 0, 3)
	closeAll := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i]()
		}
	}
	if len(diff.AddedRules)+len(diff.RemovedRules) > 0 {
		factory := opts.NewApplier
		if factory == nil {
			factory = DefaultRoutingApplierFactory
		}
		applier, closeFn, err := factory(ctx, address)
		if err != nil {
			closeAll()
			return result, func() {}, err
		}
		result.routing = applier
		if closeFn != nil {
			closers = append(closers, closeFn)
		}
	}
	if len(diff.AddedOutbounds)+len(diff.RemovedOutbounds) > 0 {
		factory := opts.NewOutbound
		if factory == nil {
			factory = DefaultOutboundApplierFactory
		}
		applier, closeFn, err := factory(ctx, address)
		if err != nil {
			closeAll()
			return result, func() {}, err
		}
		result.outbound = applier
		if closeFn != nil {
			closers = append(closers, closeFn)
		}
	}
	if len(diff.AddedInboundUsers)+len(diff.RemovedInboundUsers) > 0 {
		factory := opts.NewInboundUser
		if factory == nil {
			factory = DefaultInboundUserApplierFactory
		}
		applier, closeFn, err := factory(ctx, address)
		if err != nil {
			closeAll()
			return result, func() {}, err
		}
		result.inboundUser = applier
		if closeFn != nil {
			closers = append(closers, closeFn)
		}
	}
	return result, closeAll, nil
}

func mixedApplyPhases(diff runtimeapply.Diff) []runtimeapply.Diff {
	return []runtimeapply.Diff{
		routingRemovePhase(diff),
		outboundRemovePhase(diff),
		inboundUserRemovePhase(diff),
		inboundUserAddPhase(diff),
		outboundAddPhase(diff),
		routingAddPhase(diff),
	}
}

func applyMixedPhase(ctx context.Context, appliers mixedAppliers, diff runtimeapply.Diff) error {
	switch diff.Kind {
	case runtimeapply.DiffRoutingOnly:
		return runtimeapply.ApplyRoutingDiff(ctx, appliers.routing, diff)
	case runtimeapply.DiffOutboundOnly:
		return runtimeapply.ApplyOutboundDiff(ctx, appliers.outbound, diff)
	case runtimeapply.DiffInboundUsers:
		return runtimeapply.ApplyInboundUserDiff(ctx, appliers.inboundUser, diff)
	default:
		return nil
	}
}

func rollbackMixedPhases(ctx context.Context, appliers mixedAppliers, phases []runtimeapply.Diff) error {
	var result error
	for i := len(phases) - 1; i >= 0; i-- {
		phase := reverseMixedPhase(phases[i])
		if err := applyMixedPhase(ctx, appliers, phase); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func reverseMixedPhase(diff runtimeapply.Diff) runtimeapply.Diff {
	switch diff.Kind {
	case runtimeapply.DiffRoutingOnly:
		return reverseRoutingDiff(diff)
	case runtimeapply.DiffOutboundOnly:
		return reverseOutboundDiff(diff)
	case runtimeapply.DiffInboundUsers:
		return reverseInboundUserDiff(diff)
	default:
		return diff
	}
}

func routingRemovePhase(diff runtimeapply.Diff) runtimeapply.Diff {
	return runtimeapply.Diff{Kind: runtimeapply.DiffRoutingOnly, RemovedRuleTag: diff.RemovedRuleTag, RemovedRules: diff.RemovedRules}
}

func routingAddPhase(diff runtimeapply.Diff) runtimeapply.Diff {
	return runtimeapply.Diff{Kind: runtimeapply.DiffRoutingOnly, AddedRules: diff.AddedRules}
}

func outboundRemovePhase(diff runtimeapply.Diff) runtimeapply.Diff {
	return runtimeapply.Diff{Kind: runtimeapply.DiffOutboundOnly, RemovedOutboundTags: diff.RemovedOutboundTags, RemovedOutbounds: diff.RemovedOutbounds}
}

func outboundAddPhase(diff runtimeapply.Diff) runtimeapply.Diff {
	return runtimeapply.Diff{Kind: runtimeapply.DiffOutboundOnly, AddedOutbounds: diff.AddedOutbounds}
}

func inboundUserRemovePhase(diff runtimeapply.Diff) runtimeapply.Diff {
	return runtimeapply.Diff{Kind: runtimeapply.DiffInboundUsers, RemovedInboundUsers: diff.RemovedInboundUsers}
}

func inboundUserAddPhase(diff runtimeapply.Diff) runtimeapply.Diff {
	return runtimeapply.Diff{Kind: runtimeapply.DiffInboundUsers, AddedInboundUsers: diff.AddedInboundUsers}
}

func isEmptyDiff(diff runtimeapply.Diff) bool {
	return len(diff.AddedRules)+len(diff.RemovedRules)+len(diff.AddedOutbounds)+len(diff.RemovedOutbounds)+len(diff.AddedInboundUsers)+len(diff.RemovedInboundUsers) == 0
}
