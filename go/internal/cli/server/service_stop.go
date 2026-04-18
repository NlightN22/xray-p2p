package servercmd

import (
	"context"
	"errors"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func newServerServiceStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the xp2p server service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerServiceStop(commandContext(cmd))
			return errorForCode(code)
		},
	}
}

func runServerServiceStop(ctx context.Context) int {
	if err := common.RequireRoot(); err != nil {
		if runtime.GOOS != "windows" {
			logging.Error("xp2p server service stop requires root privileges", "err", err)
			return 1
		}
	}
	ctrl := servicecontrol.Default()
	if err := ctrl.Stop(ctx, servicecontrol.RoleServer); err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Error("xp2p server service stop is not supported on this platform")
		} else {
			logging.Error("failed to stop xp2p server service", "err", err)
		}
		return 1
	}
	logging.Info("xp2p server service stopped")
	return 0
}
