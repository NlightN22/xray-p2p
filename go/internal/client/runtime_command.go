//go:build linux || windows

package client

import (
	"context"
	"errors"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func applyClientRuntimeCandidate(ctx context.Context, artifacts xraylive.Artifacts) (xraylive.RuntimeApplyResult, error) {
	if stopped, err := serviceStopped(ctx, servicecontrol.RoleClient); err == nil && stopped {
		return xraylive.RuntimeApplyStaged, nil
	}
	liveDir, err := config.LiveRoleDir(apply.RoleClient)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	lkgDir, err := config.LkgRoleDir(apply.RoleClient)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	result, err := applyRuntimeCandidate(ctx, xraylive.Options{
		Role:    apply.RoleClient,
		LiveDir: liveDir,
		LkgDir:  lkgDir,
	}, artifacts)
	if err != nil {
		return result, err
	}
	if result == xraylive.RuntimeApplyServiceLayerRequired {
		return xraylive.RuntimeApplyStaged, nil
	}
	return result, xraylive.ResultError(result)
}

func commitClientRuntimeState(ctx context.Context, state clientInstallState) error {
	artifacts, err := compileClientRuntimeCandidate(state)
	if err != nil {
		return err
	}
	result, err := applyClientRuntimeCandidate(ctx, artifacts)
	if err != nil {
		return err
	}
	if result != xraylive.RuntimeApplyApplied && result != xraylive.RuntimeApplyNoop && result != xraylive.RuntimeApplyStaged {
		return xraylive.ResultError(result)
	}
	return state.save(config.ConfigPath(layout.ClientConfigFileName))
}

func serviceStopped(ctx context.Context, role servicecontrol.Role) (bool, error) {
	status, err := serviceStatus(ctx, role)
	if err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			return false, nil
		}
		return false, err
	}
	return !status.Active, nil
}

var serviceStatus = func(ctx context.Context, role servicecontrol.Role) (servicecontrol.Status, error) {
	return servicecontrol.Default().Status(ctx, role)
}

var applyRuntimeCandidate = xraylive.ApplyCandidate
