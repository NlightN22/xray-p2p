package clientcmd

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
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/service"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
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

func newClientServiceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show xp2p client service status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runClientServiceStatus(commandContext(cmd), cmd.OutOrStdout())
			return errorForCode(code)
		},
	}
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
		if strings.EqualFold(status.State, desired) {
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

func runClientServiceStatus(ctx context.Context, out any) int {
	ctrl := servicecontrol.Default()
	status, err := ctrl.Status(ctx, servicecontrol.RoleClient)
	if err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Error("xp2p client service status is not supported on this platform")
		} else {
			logging.Error("failed to query xp2p client service status", "err", err)
		}
		return 1
	}

	if writer, ok := out.(interface{ Write([]byte) (int, error) }); ok {
		if detail := strings.TrimSpace(status.Detail); detail != "" {
			_, _ = writer.Write([]byte(detail + "\n"))
		} else {
			text := fmt.Sprintf("xp2p client service state: %s\n", status.State)
			_, _ = writer.Write([]byte(text))
		}
	}

	if status.Active {
		return 0
	}
	return 3
}

func runClientServiceRun(ctx context.Context, cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("xp2p client service run", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	path := fs.String("path", "", "client installation directory")
	configDir := fs.String("config-dir", "", "client configuration directory name")
	logFile := fs.String("log-file", "", "xp2p service log file (default: platform-specific path)")
	maxRestarts := fs.Int("max-restarts", service.MaxRestartAttempts, "maximum restart attempts after failures")
	restartDelay := fs.Duration("restart-delay", 3*time.Second, "delay between restart attempts")
	hbEnabled := fs.Bool("heartbeat", true, "enable heartbeat probes")
	verbose := fs.Bool("verbose", false, "emit full-tunnel change details")
	hbInterval := fs.Duration("heartbeat-interval", 2*time.Second, "heartbeat interval")
	hbTimeout := fs.Duration("heartbeat-timeout", 2*time.Second, "heartbeat timeout")
	hbPort := fs.String("heartbeat-port", cfg.Server.Port, "diagnostics service port to probe")
	hbSocks := fs.String("heartbeat-socks", cfg.Client.SocksAddress, "SOCKS5 proxy for heartbeat (optional)")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		logging.Error("xp2p client service run: failed to parse arguments", "err", err)
		return 2
	}
	if fs.NArg() > 0 {
		logging.Error("xp2p client service run: unexpected arguments", "args", fs.Args())
		return 2
	}

	installDir := firstNonEmpty(*path, cfg.Client.InstallDir)
	configDirName := firstNonEmpty(*configDir, cfg.Client.ConfigDir)
	serviceLogPath := strings.TrimSpace(*logFile)
	if serviceLogPath == "" {
		serviceLogPath = defaultClientServiceLogPath(installDir)
	}
	if err := os.MkdirAll(filepath.Dir(serviceLogPath), 0o755); err != nil {
		logging.Error("xp2p client service run: failed to create log directory", "err", err)
		return 1
	}
	logWriter, err := os.OpenFile(serviceLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		logging.Error("xp2p client service run: failed to open log file", "path", serviceLogPath, "err", err)
		return 1
	}
	defer logWriter.Close()

	logging.Configure(logging.Options{Output: logWriter})

	diagPort := strings.TrimSpace(cfg.Client.DiagPort)
	if diagPort == "" {
		diagPort = cfg.Server.Port
	}

	opts := client.ServiceOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		DiagPort:     diagPort,
		MaxRestarts:  *maxRestarts,
		RestartDelay: *restartDelay,
		Heartbeat: client.HeartbeatOptions{
			Enabled:      *hbEnabled,
			Interval:     *hbInterval,
			Timeout:      *hbTimeout,
			Port:         firstNonEmpty(strings.TrimSpace(*hbPort), cfg.Server.Port),
			SocksAddress: firstNonEmpty(strings.TrimSpace(*hbSocks), cfg.Client.SocksAddress),
		},
		TunEnabled:        cfg.Client.TunEnabled,
		TunName:           cfg.Client.TunName,
		TunMTU:            cfg.Client.TunMTU,
		TunAddr:           cfg.Client.TunAddr,
		TunMode:           cfg.Client.TunMode,
		DNSServers:        cfg.Client.DNSServers,
		FullTunnelVerbose: cfg.Client.FullTunnelVerbose || *verbose,
		FullTunnelTag:     cfg.Client.FullTunnelTag,
	}

	if err := clientServiceRunFunc(ctx, opts); err != nil {
		logging.Error("xp2p client service failed", "err", err)
		return 1
	}
	return 0
}

func defaultClientServiceLogPath(installDir string) string {
	return defaultClientLogPath(installDir, "service.log")
}

func defaultClientLogPath(installDir string, fileName string) string {
	return filepath.Join(config.LogRoot(), "client", fileName)
}
