package xraylive

import (
	"context"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/runtimeapply"
	"github.com/NlightN22/xray-p2p/go/internal/xrayapi"
)

func applyOutboundRuntimeDiff(ctx context.Context, opts Options, req apply.Request, role, address string, artifacts Artifacts, diff runtimeapply.Diff) (RuntimeApplyResult, error) {
	factory := opts.NewOutbound
	if factory == nil {
		factory = DefaultOutboundApplierFactory
	}
	applier, closeApplier, err := factory(ctx, address)
	if err != nil {
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	defer func() {
		if closeApplier != nil {
			_ = closeApplier()
		}
	}()
	if err := runtimeapply.ApplyOutboundDiff(ctx, applier, diff); err != nil {
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	if err := publishLiveArtifacts(opts, artifacts); err != nil {
		rollbackErr := runtimeapply.ApplyOutboundDiff(ctx, applier, reverseOutboundDiff(diff))
		reason := err
		if rollbackErr != nil {
			reason = fmt.Errorf("%w; runtime rollback failed: %v", err, rollbackErr)
		}
		writeRuntimeApplyError(opts, req, reason)
		return RuntimeApplyFailed, nil
	}
	if err := commitDesiredOrRestoreLive(opts); err != nil {
		rollbackErr := runtimeapply.ApplyOutboundDiff(ctx, applier, reverseOutboundDiff(diff))
		reason := err
		if rollbackErr != nil {
			reason = fmt.Errorf("%w; runtime rollback failed: %v", err, rollbackErr)
		}
		writeRuntimeApplyError(opts, req, reason)
		return RuntimeApplyFailed, nil
	}

	cleanupRuntimeApplyMarkers(opts, req)
	logging.Info("runtime outbound apply completed", "role", role, "request_id", req.ID)
	return RuntimeApplyApplied, nil
}

func DefaultOutboundApplierFactory(ctx context.Context, address string) (runtimeapply.OutboundApplier, func() error, error) {
	client, err := xrayapi.Dial(ctx, address, xrayapi.DefaultTimeout)
	if err != nil {
		return nil, nil, err
	}
	return xrayOutboundApplier{client: client}, client.Close, nil
}

type xrayOutboundApplier struct {
	client *xrayapi.Client
}

func (a xrayOutboundApplier) AddOutbound(ctx context.Context, outbound map[string]any) error {
	cfg, err := xrayapi.OutboundFromMap(outbound)
	if err != nil {
		return err
	}
	return a.client.AddOutbound(ctx, cfg)
}

func (a xrayOutboundApplier) RemoveOutbound(ctx context.Context, tag string) error {
	return a.client.RemoveOutbound(ctx, tag)
}

func (a xrayOutboundApplier) ListOutboundTags(ctx context.Context) ([]string, error) {
	return a.client.ListOutboundTags(ctx)
}

func reverseOutboundDiff(diff runtimeapply.Diff) runtimeapply.Diff {
	reversed := runtimeapply.Diff{Kind: runtimeapply.DiffOutboundOnly}
	for _, change := range diff.AddedOutbounds {
		reversed.RemovedOutboundTags = append(reversed.RemovedOutboundTags, change.Tag)
		reversed.RemovedOutbounds = append(reversed.RemovedOutbounds, change)
	}
	for _, change := range diff.RemovedOutbounds {
		reversed.AddedOutbounds = append(reversed.AddedOutbounds, change)
	}
	return reversed
}
