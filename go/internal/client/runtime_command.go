//go:build linux || windows

package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

var errClientDesiredConflict = errors.New("client Desired changed concurrently; reload and retry")

func applyClientRuntimeCandidate(ctx context.Context, artifacts xraylive.Artifacts, commitDesired func() error) (xraylive.RuntimeApplyResult, error) {
	return applyClientRuntimeCandidateVerified(ctx, artifacts, commitDesired, nil)
}

func applyClientRuntimeCandidateVerified(ctx context.Context, artifacts xraylive.Artifacts, commitDesired func() error, verify func(context.Context) error) (xraylive.RuntimeApplyResult, error) {
	liveDir, err := config.LiveRoleDir(apply.RoleClient)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	lkgDir, err := config.LkgRoleDir(apply.RoleClient)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	result, err := applyRuntimeCandidate(ctx, xraylive.Options{
		Role:          apply.RoleClient,
		LiveDir:       liveDir,
		LkgDir:        lkgDir,
		CommitDesired: commitDesired,
		VerifyRuntime: verify,
	}, artifacts)
	if result == xraylive.RuntimeApplyServiceLayerRequired || result == xraylive.RuntimeApplyUnsupported ||
		result == xraylive.RuntimeApplyFailed || result == xraylive.RuntimeApplySkipped {
		if stopped, statusErr := serviceStopped(ctx, servicecontrol.RoleClient); statusErr == nil && stopped {
			return xraylive.RuntimeApplyStaged, nil
		}
	}
	if result == xraylive.RuntimeApplyServiceLayerRequired || result == xraylive.RuntimeApplyUnsupported {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	return result, xraylive.ResultError(result)
}

func commitClientRuntimeState(ctx context.Context, state clientInstallState) error {
	_, err := commitClientRuntimeStateResult(ctx, state)
	return err
}

func commitClientRuntimeStateResult(ctx context.Context, state clientInstallState) (xraylive.RuntimeApplyResult, error) {
	return commitClientRuntimeStateResultVerified(ctx, state, nil)
}

func commitClientRuntimeStateResultVerified(ctx context.Context, state clientInstallState, verify func(context.Context) error) (xraylive.RuntimeApplyResult, error) {
	return commitClientRuntimeStateResultWithCommit(ctx, state, verify, nil)
}

func commitClientRuntimeStateResultWithCommit(ctx context.Context, state clientInstallState, verify func(context.Context) error, commit func() error) (xraylive.RuntimeApplyResult, error) {
	var result xraylive.RuntimeApplyResult
	err := apply.WithRoleLock(ctx, config.StateRoot(), apply.RoleClient, func() error {
		var err error
		result, err = commitClientRuntimeStateResultLocked(ctx, state, verify, commit)
		return err
	})
	return result, err
}

func commitClientRuntimeStateResultLocked(ctx context.Context, state clientInstallState, verify func(context.Context) error, customCommit func() error) (xraylive.RuntimeApplyResult, error) {
	if state.baseDigest != "" {
		configPath := config.ConfigPath(layout.ClientConfigFileName)
		currentDigest, err := currentClientDesiredDigest(configPath)
		if err != nil {
			return xraylive.RuntimeApplySkipped, fmt.Errorf("inspect client Desired generation: %w", err)
		}
		if currentDigest != state.baseDigest {
			return xraylive.RuntimeApplySkipped, errClientDesiredConflict
		}
	}
	artifacts, err := compileClientRuntimeCandidate(state)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	desiredCommitted := false
	commitDesired := func() error {
		commit := customCommit
		if commit == nil {
			commit = func() error { return state.save(config.ConfigPath(layout.ClientConfigFileName)) }
		}
		if err := commit(); err != nil {
			return err
		}
		desiredCommitted = true
		return nil
	}
	result, err := applyClientRuntimeCandidateVerified(ctx, artifacts, commitDesired, verify)
	if err != nil {
		return result, err
	}
	if result == xraylive.RuntimeApplyServiceLayerRequired || result == xraylive.RuntimeApplyUnsupported {
		if err := commitDesired(); err != nil {
			return result, err
		}
		if err := writeClientRuntimeApplyRequest(); err != nil {
			return result, err
		}
		return xraylive.RuntimeApplyStaged, nil
	}
	if result != xraylive.RuntimeApplyApplied && result != xraylive.RuntimeApplyNoop && result != xraylive.RuntimeApplyStaged {
		return result, xraylive.ResultError(result)
	}
	if result == xraylive.RuntimeApplyApplied || result == xraylive.RuntimeApplyNoop {
		if desiredCommitted {
			return result, nil
		}
		return result, commitDesired()
	}
	return result, commitDesired()
}

func commitClientSubscriptionState(ctx context.Context, state clientInstallState) (xraylive.RuntimeApplyResult, error) {
	return commitClientRuntimeStateResult(ctx, state)
}

func commitClientSubscriptionStateVerified(ctx context.Context, state clientInstallState, verify func(context.Context) error) (xraylive.RuntimeApplyResult, error) {
	return commitClientRuntimeStateResultVerified(ctx, state, verify)
}

func commitClientSubscriptionStateTransaction(ctx context.Context, state clientInstallState, commit func() error) (xraylive.RuntimeApplyResult, error) {
	return commitClientRuntimeStateResultWithCommit(ctx, state, nil, commit)
}

func serviceStopped(ctx context.Context, role servicecontrol.Role) (bool, error) {
	status, err := serviceStatus(ctx, role)
	if err != nil {
		return true, nil
	}
	return !status.Active, nil
}

var serviceStatus = func(ctx context.Context, role servicecontrol.Role) (servicecontrol.Status, error) {
	return servicecontrol.Default().Status(ctx, role)
}

var applyRuntimeCandidate = xraylive.ApplyCandidate

func writeClientRuntimeApplyRequest() error {
	if err := apply.RemoveRoleMarkers(config.ApplyRequestPath(), config.ApplyErrorPath(), apply.RoleClient); err != nil {
		return err
	}
	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
}
