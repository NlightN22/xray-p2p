//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/ports"
)

func (m *TunnelManager) ApplyModeConfig(ctx context.Context, req ports.ModeConfigRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	switch role {
	case "client":
		return m.applyClientModeConfig(ctx, req)
	case "server":
		return m.applyServerModeConfig(ctx, req)
	default:
		return fmt.Errorf("unsupported role %q", req.Role)
	}
}

func (m *TunnelManager) applyClientModeConfig(ctx context.Context, req ports.ModeConfigRequest) error {
	if req.Mode != ports.TunnelModeSplit && req.Mode != ports.TunnelModeFull {
		return fmt.Errorf("invalid tun mode %q", req.Mode)
	}
	cfg, err := loadConfigForRole(req.ConfigPath, "client")
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Client.InstallDir) == "" {
		return errors.New("install directory is required")
	}

	fullTag := strings.TrimSpace(req.FullTag)
	if fullTag == "" {
		fullTag = strings.TrimSpace(cfg.Client.FullTunnelTag)
	}

	if _, err := config.UpdateTunEnabled(req.ConfigPath, "client", true); err != nil {
		return err
	}
	if _, err := config.UpdateTunMode(req.ConfigPath, "client", string(req.Mode)); err != nil {
		return err
	}
	if req.Mode == ports.TunnelModeFull && fullTag != "" {
		if _, err := config.UpdateFullTunnelTag(req.ConfigPath, fullTag); err != nil {
			return err
		}
	}

	reqApply, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), reqApply, config.AuditLogPath())
}

func (m *TunnelManager) applyServerModeConfig(_ context.Context, req ports.ModeConfigRequest) error {
	if _, err := config.UpdateTunEnabled(req.ConfigPath, "server", true); err != nil {
		return err
	}
	reqApply, err := apply.NewRequest(apply.RoleServer)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), reqApply, config.AuditLogPath())
}
