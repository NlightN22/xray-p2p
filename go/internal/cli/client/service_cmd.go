package clientcmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/service"
)

func newClientServiceCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the xp2p client service",
		Long: "Manage the xp2p client service.\n\n" +
			"Note: the hidden \"run\" subcommand is used by service managers " +
			"to keep the service in the foreground.",
	}

	cmd.AddCommand(
		newClientServiceStartCmd(),
		newClientServiceStopCmd(),
		newClientServiceRestartCmd(),
		newClientServiceStatusCmd(),
		newClientServiceRunCmd(cfg),
	)
	return cmd
}

func newClientServiceRunCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the xp2p client service in the foreground",
		Long:  "Run the xp2p client service in the foreground (intended for service managers).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if logLevel, ok, err := common.LogLevelFromFlags(cmd); err != nil {
				return err
			} else if ok {
				if err := common.ApplyProcessLogLevel(logLevel); err != nil {
					logging.Error("xp2p client service run: invalid --log-level", "err", err)
					return errorForCode(2)
				}
			}
			forwarded := forwardFlags(cmd, args)
			code := runClientServiceRun(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringP("path", "p", "", "client installation directory")
	flags.StringP("config-dir", "D", "", "client configuration directory name")
	flags.StringP("log-file", "F", "", "xp2p service log file (default: platform-specific path)")
	flags.IntP("max-restarts", "R", service.MaxRestartAttempts, "maximum restart attempts after failures")
	flags.DurationP("restart-delay", "r", 3*time.Second, "delay between restart attempts")
	flags.BoolP("heartbeat", "b", true, "enable heartbeat probes")
	flags.BoolP("verbose", "V", false, "emit full-tunnel change details")
	flags.DurationP("heartbeat-interval", "I", 2*time.Second, "heartbeat interval")
	flags.DurationP("heartbeat-timeout", "T", 2*time.Second, "heartbeat timeout")
	flags.StringP("heartbeat-port", "P", "", "diagnostics service port to probe")
	flags.StringP("heartbeat-socks", "S", "", "SOCKS5 proxy for heartbeat (optional)")
	return cmd
}
