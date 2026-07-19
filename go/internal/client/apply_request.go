//go:build linux || windows

package client

import (
	"context"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func applyPendingIfRequested(role string) (*apply.Rollback, bool, apply.Request, error) {
	var rollback *apply.Rollback
	var applied bool
	var req apply.Request
	err := apply.WithRoleLock(context.Background(), config.StateRoot(), role, func() error {
		var err error
		rollback, applied, req, err = applyPendingIfRequestedLocked(role)
		return err
	})
	return rollback, applied, req, err
}

func applyPendingIfRequestedLocked(role string) (*apply.Rollback, bool, apply.Request, error) {
	reqPath := config.ApplyRequestPath()
	desiredConfigPath, desiredPathErr := config.DesiredConfigPathForRole(role)
	logging.Debug("apply request check",
		"role", role,
		"apply_request", reqPath,
		"apply_request_exists", fileExists(reqPath),
		"state_root", config.StateRoot(),
		"state_root_exists", dirExists(config.StateRoot()),
		"desired_config", desiredConfigPath,
		"desired_config_exists", fileExists(desiredConfigPath),
		"desired_config_error", desiredPathErr,
	)
	req, exists, err := apply.ReadRequestForRole(reqPath, role)
	if err != nil {
		return nil, false, apply.Request{}, err
	}
	if !exists {
		return nil, false, apply.Request{}, nil
	}
	if !req.MatchesRole(role) {
		return nil, false, apply.Request{}, nil
	}
	errorPath := config.ApplyErrorPath()
	if marker, markerExists, err := apply.ReadError(errorPath); err != nil {
		return nil, false, apply.Request{}, err
	} else if markerExists && marker.RequestID != "" && marker.RequestID == req.ID {
		logging.Warn("apply request skipped (previous failure)", "role", role, "request_id", req.ID, "reason", marker.Reason)
		return nil, false, req, nil
	}

	desiredConfig, err := config.DesiredConfigPathForRole(role)
	if err != nil {
		return nil, false, req, err
	}
	extensionsDir, err := config.DesiredExtensionsDirForRole(role)
	if err != nil {
		return nil, false, req, err
	}
	liveDir, err := config.LiveRoleDir(role)
	if err != nil {
		return nil, false, req, err
	}
	lkgDir, err := config.LkgRoleDir(role)
	if err != nil {
		return nil, false, req, err
	}
	sourceDigest, err := apply.SourceDigest(desiredConfig, extensionsDir)
	if err != nil {
		return nil, false, req, err
	}
	artifacts, err := compileDesired(desiredConfig, extensionsDir)
	if err != nil {
		_ = apply.WriteError(errorPath, apply.ErrorMarker{
			RequestID: req.ID,
			Role:      role,
			Reason:    err.Error(),
		}, config.AuditLogPath())
		logging.Warn("apply compilation failed", "role", role, "request_id", req.ID, "err", err)
		return nil, false, req, nil
	}
	currentDigest, err := apply.SourceDigest(desiredConfig, extensionsDir)
	if err != nil {
		return nil, false, req, err
	}
	if currentDigest != sourceDigest {
		logging.Info("desired config changed during compilation; apply request retained", "role", role, "request_id", req.ID)
		return nil, false, req, nil
	}
	files := map[string][]byte{
		layout.XrayConfigFileName:  artifacts.XrayJSON,
		layout.RuntimeMetaFileName: artifacts.MetaJSON,
	}
	for name, data := range liveSelectorExtras(liveDir) {
		files[name] = data
	}
	if err := apply.ReplaceRoleLiveDir(liveDir, lkgDir, files); err != nil {
		_ = apply.WriteError(errorPath, apply.ErrorMarker{
			RequestID: req.ID,
			Role:      role,
			Reason:    err.Error(),
		}, config.AuditLogPath())
		logging.Warn("apply write failed", "role", role, "request_id", req.ID, "err", err)
		return nil, false, req, nil
	}

	if err := apply.CompleteRequest(reqPath, errorPath, req); err != nil {
		logging.Warn("apply marker cleanup failed", "role", role, "err", err)
	}
	logging.Info("desired config compiled into live artifacts", "role", role, "request_id", req.ID, "live_dir", liveDir)
	return apply.NewRollback(liveDir, lkgDir), true, req, nil
}

func tryRuntimeApplyPending(ctx context.Context, role string) (xraylive.RuntimeApplyResult, error) {
	var result xraylive.RuntimeApplyResult
	err := apply.WithRoleLock(ctx, config.StateRoot(), role, func() error {
		var err error
		result, err = tryRuntimeApplyPendingLocked(ctx, role)
		return err
	})
	return result, err
}

func tryRuntimeApplyPendingLocked(ctx context.Context, role string) (xraylive.RuntimeApplyResult, error) {
	desiredConfig, err := config.DesiredConfigPathForRole(role)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	extensionsDir, err := config.DesiredExtensionsDirForRole(role)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	liveDir, err := config.LiveRoleDir(role)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	lkgDir, err := config.LkgRoleDir(role)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	return xraylive.TryApplyRoutingPending(ctx, xraylive.Options{
		Role:          role,
		RequestPath:   config.ApplyRequestPath(),
		ErrorPath:     config.ApplyErrorPath(),
		AuditPath:     config.AuditLogPath(),
		DesiredConfig: desiredConfig,
		ExtensionsDir: extensionsDir,
		LiveDir:       liveDir,
		LkgDir:        lkgDir,
		SourceDigest:  apply.SourceDigest,
		Compile: func(configPath, extensionsDir string) (xraylive.Artifacts, error) {
			artifacts, err := compileDesired(configPath, extensionsDir)
			if err != nil {
				return xraylive.Artifacts{}, err
			}
			return xraylive.Artifacts{
				XrayJSON: artifacts.XrayJSON,
				MetaJSON: artifacts.MetaJSON,
				Extra:    liveSelectorExtras(liveDir),
			}, nil
		},
	})
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func liveSelectorExtras(liveDir string) map[string][]byte {
	extra := map[string][]byte{}
	for _, name := range []string{
		layout.ClientEndpointSelectorStateFileName,
		layout.ClientEndpointSelectorJournalFileName,
	} {
		data, err := os.ReadFile(filepath.Join(liveDir, name))
		if err == nil {
			extra[name] = data
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}
