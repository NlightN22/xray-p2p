package servercmd

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

func newServerServiceRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the xp2p server service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerServiceRestart(commandContext(cmd))
			return errorForCode(code)
		},
	}
}

func runServerServiceRestart(ctx context.Context) int {
	if err := common.RequireRoot(); err != nil {
		if runtime.GOOS != "windows" {
			logging.Error("xp2p server service restart requires root privileges", "err", err)
			return 1
		}
	}
	ctrl := servicecontrol.Default()
	if err := ctrl.Stop(ctx, servicecontrol.RoleServer); err != nil && !errors.Is(err, servicecontrol.ErrUnsupported) {
		logging.Error("failed to stop xp2p server service", "err", err)
		return 1
	}
	if err := waitForServiceState(ctx, ctrl, servicecontrol.RoleServer, "STOPPED", 60*time.Second); err != nil {
		logging.Error("xp2p server service restart: stop timed out", "err", err)
		return 1
	}
	if err := ctrl.Start(ctx, servicecontrol.RoleServer); err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Error("xp2p server service restart is not supported on this platform")
		} else {
			logging.Error("failed to start xp2p server service", "err", err)
		}
		return 1
	}
	if err := waitForServiceState(ctx, ctrl, servicecontrol.RoleServer, "RUNNING", 60*time.Second); err != nil {
		logging.Error("xp2p server service restart: start timed out", "err", err)
		return 1
	}
	logging.Info("xp2p server service restarted")
	return 0
}
