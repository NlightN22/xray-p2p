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

func newClientServiceStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the xp2p client service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			logLevel, logLevelSet, err := common.LogLevelFromFlags(cmd)
			if err != nil {
				return err
			}
			code := runClientServiceStart(commandContext(cmd), logLevel, logLevelSet)
			return errorForCode(code)
		},
	}
}

func runClientServiceStart(ctx context.Context, logLevel string, logLevelSet bool) int {
	if err := common.RequireRoot(); err != nil {
		if runtime.GOOS != "windows" {
			logging.Error("xp2p client service start requires root privileges", "err", err)
			return 1
		}
	}
	if logLevelSet {
		normalized, err := logging.NormalizeLevel(logLevel)
		if err != nil {
			logging.Error("xp2p client service start: invalid --log-level", "err", err)
			return 2
		}
		if err := servicecontrol.SetServiceEnv(ctx, servicecontrol.RoleClient, map[string]string{logging.EnvLogLevel: normalized}); err != nil && !errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Error("xp2p client service start: failed to update service environment", "err", err)
			return 1
		}
	}
	ctrl := servicecontrol.Default()
	if err := ctrl.Start(ctx, servicecontrol.RoleClient); err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Error("xp2p client service start is not supported on this platform")
		} else {
			logging.Error("failed to start xp2p client service", "err", err)
		}
		return 1
	}
	logging.Info("xp2p client service started")
	return 0
}
