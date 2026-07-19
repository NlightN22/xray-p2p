package xraylive

import (
	"context"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/runtimeapply"
	"github.com/NlightN22/xray-p2p/go/internal/xrayapi"
)

func applyInboundUserRuntimeDiff(ctx context.Context, opts Options, req apply.Request, role, address string, artifacts Artifacts, diff runtimeapply.Diff) (RuntimeApplyResult, error) {
	factory := opts.NewInboundUser
	if factory == nil {
		factory = DefaultInboundUserApplierFactory
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
	if err := runtimeapply.ApplyInboundUserDiff(ctx, applier, diff); err != nil {
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	if err := publishLiveArtifacts(opts, artifacts); err != nil {
		rollbackErr := runtimeapply.ApplyInboundUserDiff(ctx, applier, reverseInboundUserDiff(diff))
		reason := err
		if rollbackErr != nil {
			reason = fmt.Errorf("%w; runtime rollback failed: %v", err, rollbackErr)
		}
		writeRuntimeApplyError(opts, req, reason)
		return RuntimeApplyFailed, nil
	}
	if err := commitDesiredOrRestoreLive(opts); err != nil {
		rollbackErr := runtimeapply.ApplyInboundUserDiff(ctx, applier, reverseInboundUserDiff(diff))
		reason := err
		if rollbackErr != nil {
			reason = fmt.Errorf("%w; runtime rollback failed: %v", err, rollbackErr)
		}
		writeRuntimeApplyError(opts, req, reason)
		return RuntimeApplyFailed, nil
	}

	cleanupRuntimeApplyMarkers(opts, req)
	logging.Info("runtime inbound user apply completed", "role", role, "request_id", req.ID)
	return RuntimeApplyApplied, nil
}

func DefaultInboundUserApplierFactory(ctx context.Context, address string) (runtimeapply.InboundUserApplier, func() error, error) {
	client, err := xrayapi.Dial(ctx, address, xrayapi.DefaultTimeout)
	if err != nil {
		return nil, nil, err
	}
	return xrayInboundUserApplier{client: client}, client.Close, nil
}

type xrayInboundUserApplier struct {
	client *xrayapi.Client
}

func (a xrayInboundUserApplier) AddInboundUser(ctx context.Context, inboundTag, email, password string) error {
	return a.client.AddInboundUser(ctx, inboundTag, email, password)
}

func (a xrayInboundUserApplier) RemoveInboundUser(ctx context.Context, inboundTag, email string) error {
	return a.client.RemoveInboundUser(ctx, inboundTag, email)
}

func (a xrayInboundUserApplier) ListInboundUserEmails(ctx context.Context, inboundTag string) ([]string, error) {
	return a.client.ListInboundUserEmails(ctx, inboundTag)
}

func reverseInboundUserDiff(diff runtimeapply.Diff) runtimeapply.Diff {
	reversed := runtimeapply.Diff{Kind: runtimeapply.DiffInboundUsers}
	for _, change := range diff.AddedInboundUsers {
		reversed.RemovedInboundUsers = append(reversed.RemovedInboundUsers, change)
	}
	for _, change := range diff.RemovedInboundUsers {
		reversed.AddedInboundUsers = append(reversed.AddedInboundUsers, change)
	}
	return reversed
}
