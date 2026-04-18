package servercmd

import (
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/service"
)

func newServerServiceCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the xp2p server service",
		Long: "Manage the xp2p server service.\n\n" +
			"Note: the hidden \"run\" subcommand is used by service managers " +
			"to keep the service in the foreground.",
	}
	cmd.AddCommand(
		newServerServiceStartCmd(),
		newServerServiceStopCmd(),
		newServerServiceRestartCmd(),
		newServerServiceStatusCmd(),
		newServerServiceRunCmd(cfg),
	)
	return cmd
}

func newServerServiceRunCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the xp2p server service in the foreground",
		Long:  "Run the xp2p server service in the foreground (intended for service managers).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if logLevel, ok, err := common.LogLevelFromFlags(cmd); err != nil {
				return err
			} else if ok {
				if err := common.ApplyProcessLogLevel(logLevel); err != nil {
					logging.Error("xp2p server service run: invalid --log-level", "err", err)
					return errorForCode(2)
				}
			}
			forwarded := forwardFlags(cmd, args)
			code := runServerServiceRun(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringP("path", "p", "", "server installation directory")
	flags.StringP("config-dir", "D", "", "server configuration directory name")
	flags.StringP("diag-service-port", "P", "", "diagnostics service port")
	flags.StringP("diag-service-mode", "M", "", "diagnostics service startup mode (auto|manual)")
	flags.StringP("log-file", "F", filepath.Join(config.LogRoot(), "server", "service.log"), "xp2p service log file")
	flags.IntP("max-restarts", "R", service.MaxRestartAttempts, "maximum restart attempts after failures")
	flags.DurationP("restart-delay", "r", 3*time.Second, "delay between restart attempts")
	return cmd
}
