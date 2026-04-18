package clientcmd

import (
	"context"
	"errors"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func newClientServiceRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the xp2p client service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runClientServiceRestart(commandContext(cmd))
			return errorForCode(code)
		},
	}
}

func runClientServiceRestart(ctx context.Context) int {
	if err := common.RequireRoot(); err != nil {
		if runtime.GOOS != "windows" {
			logging.Error("xp2p client service restart requires root privileges", "err", err)
			return 1
		}
	}
	ctrl := servicecontrol.Default()
	if err := ctrl.Stop(ctx, servicecontrol.RoleClient); err != nil && !errors.Is(err, servicecontrol.ErrUnsupported) {
		logging.Error("failed to stop xp2p client service", "err", err)
		return 1
	}
	if err := waitForServiceState(ctx, ctrl, servicecontrol.RoleClient, "STOPPED", 60*time.Second); err != nil {
		logging.Error("xp2p client service restart: stop timed out", "err", err)
		return 1
	}
	if err := ctrl.Start(ctx, servicecontrol.RoleClient); err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Error("xp2p client service restart is not supported on this platform")
		} else {
			logging.Error("failed to start xp2p client service", "err", err)
		}
		return 1
	}
	if err := waitForServiceState(ctx, ctrl, servicecontrol.RoleClient, "RUNNING", 60*time.Second); err != nil {
		logging.Error("xp2p client service restart: start timed out", "err", err)
		return 1
	}
	logging.Info("xp2p client service restarted")
	return 0
}
