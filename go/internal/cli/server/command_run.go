package servercmd

import (
	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func newServerRunCmd(cfg commandConfig) *cobra.Command {
	var opts serverRunCommandOptions
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run xp2p server in foreground",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if logLevel, ok, err := common.LogLevelFromFlags(cmd); err != nil {
				return err
			} else if ok {
				if err := common.ApplyProcessLogLevel(logLevel); err != nil {
					logging.Error("xp2p server run: invalid --log-level", "err", err)
					return errorForCode(2)
				}
			}
			code := runServerRun(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.StringVarP(&opts.ConfigDir, "config-dir", "D", "", "server configuration directory name")
	flags.StringVarP(&opts.DiagPort, "diag-service-port", "P", "", "diagnostics service port")
	flags.StringVarP(&opts.DiagMode, "diag-service-mode", "M", "", "diagnostics service startup mode (auto|manual)")
	flags.BoolVarP(&opts.AutoInstall, "auto-install", "A", false, "install server assets when missing without prompting")
	flags.BoolVarP(&opts.Quiet, "quiet", "q", false, "suppress interactive prompts")
	return cmd
}
