package clientcmd

import (
	"context"
	"errors"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func newClientServiceStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the xp2p client service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runClientServiceStop(commandContext(cmd))
			return errorForCode(code)
		},
	}
}

func runClientServiceStop(ctx context.Context) int {
	if err := common.RequireRoot(); err != nil {
		if runtime.GOOS != "windows" {
			logging.Error("xp2p client service stop requires root privileges", "err", err)
			return 1
		}
	}
	ctrl := servicecontrol.Default()
	if err := ctrl.Stop(ctx, servicecontrol.RoleClient); err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Error("xp2p client service stop is not supported on this platform")
		} else {
			logging.Error("failed to stop xp2p client service", "err", err)
		}
		return 1
	}
	logging.Info("xp2p client service stopped")
	return 0
}
