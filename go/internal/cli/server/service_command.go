package servercmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/service"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
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
		newServerServiceStatusCmd(),
		newServerServiceRunCmd(cfg),
	)
	return cmd
}

func newServerServiceStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the xp2p server service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerServiceStart(commandContext(cmd))
			return errorForCode(code)
		},
	}
}

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

func newServerServiceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show xp2p server service status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerServiceStatus(commandContext(cmd), cmd.OutOrStdout())
			return errorForCode(code)
		},
	}
}

func newServerServiceRunCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the xp2p server service in the foreground",
		Long:  "Run the xp2p server service in the foreground (intended for service managers).",
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runServerServiceRun(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "server installation directory")
	flags.String("config-dir", "", "server configuration directory name")
	flags.String("log-file", filepath.Join(config.LogRoot(), "server", "service.log"), "xp2p service log file")
	flags.String("xray-log-file", filepath.Join(config.LogRoot(), "server", "xray-service.log"), "xray stderr log file")
	flags.Int("max-restarts", service.MaxRestartAttempts, "maximum restart attempts after failures")
	flags.Duration("restart-delay", 3*time.Second, "delay between restart attempts")
	return cmd
}

func runServerServiceStart(ctx context.Context) int {
	if err := common.RequireRoot(); err != nil {
		logging.Error("xp2p server service start requires root privileges", "err", err)
		return 1
	}
	ctrl := servicecontrol.Default()
	if err := ctrl.Start(ctx, servicecontrol.RoleServer); err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Error("xp2p server service start is not supported on this platform")
		} else {
			logging.Error("failed to start xp2p server service", "err", err)
		}
		return 1
	}
	logging.Info("xp2p server service started")
	return 0
}

func runServerServiceStop(ctx context.Context) int {
	if err := common.RequireRoot(); err != nil {
		logging.Error("xp2p server service stop requires root privileges", "err", err)
		return 1
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

func runServerServiceStatus(ctx context.Context, out any) int {
	ctrl := servicecontrol.Default()
	status, err := ctrl.Status(ctx, servicecontrol.RoleServer)
	if err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Error("xp2p server service status is not supported on this platform")
		} else {
			logging.Error("failed to query xp2p server service status", "err", err)
		}
		return 1
	}

	if writer, ok := out.(interface{ Write([]byte) (int, error) }); ok {
		if detail := strings.TrimSpace(status.Detail); detail != "" {
			_, _ = writer.Write([]byte(detail + "\n"))
		} else {
			text := fmt.Sprintf("xp2p server service state: %s\n", status.State)
			_, _ = writer.Write([]byte(text))
		}
	}
	if status.Active {
		return 0
	}
	return 3
}

func runServerServiceRun(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p server service run", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "server installation directory")
	configDir := fs.String("config-dir", "", "server configuration directory name")
	logFile := fs.String("log-file", "", "xp2p service log file (default: platform-specific path)")
	xrayLog := fs.String("xray-log-file", "", "xray stderr log file (default: platform-specific path)")
	maxRestarts := fs.Int("max-restarts", service.MaxRestartAttempts, "maximum restart attempts after failures")
	restartDelay := fs.Duration("restart-delay", 3*time.Second, "delay between restart attempts")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p server service run: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p server service run: unexpected arguments", "args", fs.Args())
		return 2
	}

	installDir := common.FirstNonEmpty(*path, cfg.Server.InstallDir)
	configDirName := common.FirstNonEmpty(*configDir, cfg.Server.ConfigDir)
	serviceLogPath := strings.TrimSpace(*logFile)
	if serviceLogPath == "" {
		serviceLogPath = defaultServerServiceLogPath(installDir)
	}
	if err := os.MkdirAll(filepath.Dir(serviceLogPath), 0o755); err != nil {
		logging.Error("xp2p server service run: failed to create log directory", "err", err)
		return 1
	}
	logWriter, err := os.OpenFile(serviceLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		logging.Error("xp2p server service run: failed to open log file", "path", serviceLogPath, "err", err)
		return 1
	}
	defer logWriter.Close()
	logging.Configure(logging.Options{Output: logWriter})

	xrayLogPath := strings.TrimSpace(*xrayLog)
	if xrayLogPath == "" {
		xrayLogPath = defaultServerXrayLogPath(installDir)
	}

	opts := server.ServiceOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		XrayLogPath:  xrayLogPath,
		DiagPort:     cfg.Server.Port,
		MaxRestarts:  *maxRestarts,
		RestartDelay: *restartDelay,
		TunEnabled:   cfg.Server.TunEnabled,
		TunName:      cfg.Server.TunName,
		TunMTU:       cfg.Server.TunMTU,
		TunAddr:      cfg.Server.TunAddr,
	}
	if err := serverServiceRunFunc(ctx, opts); err != nil {
		logging.Error("xp2p server service failed", "err", err)
		return 1
	}
	return 0
}

func defaultServerServiceLogPath(installDir string) string {
	return defaultServerLogPath(installDir, "service.log")
}

func defaultServerXrayLogPath(installDir string) string {
	return defaultServerLogPath(installDir, "xray-service.log")
}

func defaultServerLogPath(installDir string, fileName string) string {
	if runtime.GOOS == "windows" {
		base := strings.TrimSpace(installDir)
		if base == "" {
			base = "."
		}
		return filepath.Join(base, layout.LogsDirName, "server", fileName)
	}
	return filepath.Join(config.LogRoot(), "server", fileName)
}
