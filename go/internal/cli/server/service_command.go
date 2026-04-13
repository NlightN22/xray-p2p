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
		newServerServiceRestartCmd(),
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
			logLevel, logLevelSet, err := common.LogLevelFromFlags(cmd)
			if err != nil {
				return err
			}
			code := runServerServiceStart(commandContext(cmd), logLevel, logLevelSet)
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

func runServerServiceStart(ctx context.Context, logLevel string, logLevelSet bool) int {
	if err := common.RequireRoot(); err != nil {
		if runtime.GOOS != "windows" {
			logging.Error("xp2p server service start requires root privileges", "err", err)
			return 1
		}
	}
	if logLevelSet {
		normalized, err := logging.NormalizeLevel(logLevel)
		if err != nil {
			logging.Error("xp2p server service start: invalid --log-level", "err", err)
			return 2
		}
		if err := servicecontrol.SetServiceEnv(ctx, servicecontrol.RoleServer, map[string]string{logging.EnvLogLevel: normalized}); err != nil && !errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Error("xp2p server service start: failed to update service environment", "err", err)
			return 1
		}
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

func waitForServiceState(ctx context.Context, ctrl servicecontrol.Controller, role servicecontrol.Role, desired string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		status, err := ctrl.Status(ctx, role)
		if err != nil {
			if errors.Is(err, servicecontrol.ErrUnsupported) {
				return err
			}
			return err
		}
		if serviceStateMatches(status.State, desired) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("service %s did not reach %s (state=%s)", role, desired, status.State)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func serviceStateMatches(state, desired string) bool {
	state = strings.ToLower(strings.TrimSpace(state))
	desired = strings.ToLower(strings.TrimSpace(desired))
	switch desired {
	case "stopped":
		return state == "inactive" || state == "failed" || state == "dead" || state == "stopped"
	case "running":
		return state == "active" || state == "running"
	default:
		return state == desired
	}
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
	diagPort := fs.String("diag-service-port", "", "diagnostics service port")
	diagMode := fs.String("diag-service-mode", "", "diagnostics service startup mode (auto|manual)")
	logFile := fs.String("log-file", "", "xp2p service log file (default: platform-specific path)")
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
	if mode := strings.TrimSpace(*diagMode); mode != "" {
		cfg.Server.Mode = mode
	}
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

	opts := server.ServiceOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		DiagPort:     common.FirstNonEmpty(strings.TrimSpace(*diagPort), cfg.Server.Port),
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

func defaultServerLogPath(installDir string, fileName string) string {
	return filepath.Join(config.LogRoot(), "server", fileName)
}
