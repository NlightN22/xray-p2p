//go:build windows || linux

package server

import (
	"context"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/runtimeboundary"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func applyServerRuntimeCandidate(ctx context.Context, artifacts xraylive.Artifacts, commitDesired func() error) (xraylive.RuntimeApplyResult, error) {
	liveDir, err := config.LiveRoleDir(apply.RoleServer)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	lkgDir, err := config.LkgRoleDir(apply.RoleServer)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	result, err := runtimeboundary.ApplyCandidate(ctx, xraylive.Options{
		Role:          apply.RoleServer,
		LiveDir:       liveDir,
		LkgDir:        lkgDir,
		CommitDesired: commitDesired,
	}, artifacts)
	if result == xraylive.RuntimeApplyServiceLayerRequired && !serverLiveRuntimeAvailable(liveDir) {
		return xraylive.RuntimeApplyStaged, nil
	}
	if result == xraylive.RuntimeApplyServiceLayerRequired || result == xraylive.RuntimeApplyUnsupported ||
		result == xraylive.RuntimeApplyFailed || result == xraylive.RuntimeApplySkipped {
		if stopped, statusErr := serviceStopped(ctx, servicecontrol.RoleServer); statusErr == nil && stopped {
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

func serverLiveRuntimeAvailable(liveDir string) bool {
	for _, name := range []string{layout.XrayConfigFileName, layout.RuntimeMetaFileName} {
		info, err := os.Stat(filepath.Join(liveDir, name))
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func commitServerRuntimeDoc(ctx context.Context, doc map[string]any) error {
	_, err := commitServerRuntimeDocResult(ctx, doc)
	return err
}

func commitServerRuntimeDocResult(ctx context.Context, doc map[string]any) (xraylive.RuntimeApplyResult, error) {
	var result xraylive.RuntimeApplyResult
	err := apply.WithRoleLock(ctx, config.StateRoot(), apply.RoleServer, func() error {
		var err error
		result, err = commitServerRuntimeDocResultLocked(ctx, doc)
		return err
	})
	return result, err
}

func commitServerRuntimeDocResultLocked(ctx context.Context, doc map[string]any) (xraylive.RuntimeApplyResult, error) {
	artifacts, err := compileServerRuntimeCandidateDoc(doc)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	desiredCommitted := false
	commitDesired := func() error {
		if err := writeServerStateDoc(pendingConfigPath(), doc); err != nil {
			return err
		}
		desiredCommitted = true
		return nil
	}
	result, err := applyServerRuntimeCandidate(ctx, artifacts, commitDesired)
	if err != nil {
		return result, err
	}
	if result == xraylive.RuntimeApplyServiceLayerRequired || result == xraylive.RuntimeApplyUnsupported {
		if err := writeServerStateDoc(pendingConfigPath(), doc); err != nil {
			return result, err
		}
		if err := writeServerRuntimeApplyRequest(); err != nil {
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
	return result, writeServerStateDoc(pendingConfigPath(), doc)
}

func compileServerRuntimeCandidateDoc(doc map[string]any) (xraylive.Artifacts, error) {
	sourcePath := pendingConfigPath()
	file, err := os.CreateTemp("", "xp2p-server-candidate-*.toml")
	if err != nil {
		return xraylive.Artifacts{}, err
	}
	candidatePath := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(candidatePath)
		return xraylive.Artifacts{}, err
	}
	defer os.Remove(candidatePath)
	if data, err := os.ReadFile(sourcePath); err == nil {
		if err := os.WriteFile(candidatePath, data, 0o644); err != nil {
			return xraylive.Artifacts{}, err
		}
	} else if !os.IsNotExist(err) {
		return xraylive.Artifacts{}, err
	}
	if err := writeServerStateDoc(candidatePath, doc); err != nil {
		return xraylive.Artifacts{}, err
	}
	extensionsDir, err := config.DesiredExtensionsDirForRole(apply.RoleServer)
	if err != nil {
		return xraylive.Artifacts{}, err
	}
	artifacts, err := compileDesired(candidatePath, extensionsDir)
	if err != nil {
		return xraylive.Artifacts{}, err
	}
	extra := make(map[string][]byte, len(artifacts.Extra))
	for name, data := range artifacts.Extra {
		extra[filepath.Clean(name)] = data
	}
	return xraylive.Artifacts{XrayJSON: artifacts.XrayJSON, MetaJSON: artifacts.MetaJSON, Extra: extra}, nil
}

func serviceStopped(ctx context.Context, role servicecontrol.Role) (bool, error) {
	status, err := runtimeboundary.ServiceStatus(ctx, role)
	if err != nil {
		return true, nil
	}
	return !status.Active, nil
}

func writeServerRuntimeApplyRequest() error {
	if err := apply.RemoveRoleMarkers(config.ApplyRequestPath(), config.ApplyErrorPath(), apply.RoleServer); err != nil {
		return err
	}
	req, err := apply.NewRequest(apply.RoleServer)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
}
