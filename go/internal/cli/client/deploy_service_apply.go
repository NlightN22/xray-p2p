package clientcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/health"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func ensureClientServiceApplied(ctx context.Context, socksAddr string) (bool, error) {
	ctrl := servicecontrol.Default()
	status, err := ctrl.Status(ctx, servicecontrol.RoleClient)
	if err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			return false, err
		}
		return false, err
	}
	if !status.Active {
		return false, nil
	}
	if err := waitForApplyRequestClear(ctx, config.ApplyRequestPath(), applyRequestTimeout); err != nil {
		logDeployPaths("xp2p client deploy: apply request timeout", config.LiveConfigPath(layout.ClientConfigFileName))
		return status.Active, err
	}
	if strings.TrimSpace(socksAddr) == "" {
		return status.Active, nil
	}
	if err := health.WaitForSocksProxy(ctx, socksAddr, socksReadyTimeout, socksProbeInterval); err != nil {
		return status.Active, err
	}
	return status.Active, nil
}

func logClientServiceApplyHint(ctx context.Context) {
	ctrl := servicecontrol.Default()
	status, err := ctrl.Status(ctx, servicecontrol.RoleClient)
	if err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Warn("xp2p client deploy: service manager unavailable; start or restart service to apply pending changes")
			return
		}
		logging.Warn("xp2p client deploy: service status check failed", "err", err)
		return
	}
	if status.Active {
		logging.Info("xp2p client deploy: service active; restart required to apply pending changes")
	} else {
		logging.Info("xp2p client deploy: service inactive; start required to apply pending changes")
	}
}

func waitForApplyRequestClear(ctx context.Context, path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("apply request still present after %s", timeout)
}
