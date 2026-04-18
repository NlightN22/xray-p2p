package servercmd

import (
	"errors"
	"os"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func ensureServerApplyRequestIfDesiredOnly() error {
	liveConfigDir, err := config.LiveRoleDir(apply.RoleServer)
	if err != nil {
		return err
	}
	livePresent, err := configFilesPresent(liveConfigDir, requiredServerArtifacts)
	if err != nil {
		return err
	}
	if livePresent {
		return nil
	}

	desiredPresent, err := desiredInputsPresent(apply.RoleServer)
	if err != nil {
		return err
	}
	if !desiredPresent {
		return nil
	}

	applyPath := config.ApplyRequestPath()
	if _, err := os.Stat(applyPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	req, err := apply.NewRequest(apply.RoleServer)
	if err != nil {
		return err
	}
	if err := apply.WriteRequest(applyPath, req, config.AuditLogPath()); err != nil {
		return err
	}
	logging.Info("xp2p server run: apply request recorded for pending-only configuration",
		"desired_config", config.ConfigPath(layout.ServerConfigFileName),
		"extensions_dir", config.ConfigPath(layout.ServerConfigDir),
	)
	return nil
}

func desiredInputsPresent(role string) (bool, error) {
	desiredConfig, err := config.DesiredConfigPathForRole(role)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(desiredConfig); err == nil {
		return true, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return false, nil
}
