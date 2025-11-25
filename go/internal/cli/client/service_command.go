package clientcmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/service"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func newClientServiceCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the xp2p client service",
	}

	cmd.AddCommand(
		newClientServiceStartCmd(),
		newClientServiceStopCmd(),
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
			code := runClientServiceStart(commandContext(cmd))
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
		Use:    "run",
		Short:  "Run the xp2p client service in the foreground",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			forwarded := forwardFlags(cmd, args)
			code := runClientServiceRun(commandContext(cmd), cfg(), forwarded)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.String("path", "", "client installation directory")
	flags.String("config-dir", "", "client configuration directory name")
	flags.String("log-file", filepath.Join(layout.UnixLogRoot, "client", "service.log"), "xp2p service log file")
	flags.String("xray-log-file", filepath.Join(layout.UnixLogRoot, "client", "xray-service.log"), "xray stderr log file")
	flags.Int("max-restarts", service.MaxRestartAttempts, "maximum restart attempts after failures")
	flags.Duration("restart-delay", 3*time.Second, "delay between restart attempts")
	flags.Bool("heartbeat", true, "enable heartbeat probes")
	flags.Duration("heartbeat-interval", 2*time.Second, "heartbeat interval")
	flags.Duration("heartbeat-timeout", 2*time.Second, "heartbeat timeout")
	flags.String("heartbeat-port", "", "diagnostics service port to probe")
	flags.String("heartbeat-socks", "", "SOCKS5 proxy for heartbeat (optional)")
	return cmd
}

func runClientServiceStart(ctx context.Context) int {
	if err := common.RequireRoot(); err != nil {
		logging.Error("xp2p client service start requires root privileges", "err", err)
		return 1
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
		logging.Error("xp2p client service stop requires root privileges", "err", err)
		return 1
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
	logFile := fs.String("log-file", filepath.Join(layout.UnixLogRoot, "client", "service.log"), "xp2p service log file")
	xrayLog := fs.String("xray-log-file", filepath.Join(layout.UnixLogRoot, "client", "xray-service.log"), "xray stderr log file")
	maxRestarts := fs.Int("max-restarts", service.MaxRestartAttempts, "maximum restart attempts after failures")
	restartDelay := fs.Duration("restart-delay", 3*time.Second, "delay between restart attempts")
	hbEnabled := fs.Bool("heartbeat", true, "enable heartbeat probes")
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
		serviceLogPath = filepath.Join(layout.UnixLogRoot, "client", "service.log")
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

	opts := client.ServiceOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		XrayLogPath:  strings.TrimSpace(*xrayLog),
		DiagPort:     cfg.Server.Port,
		MaxRestarts:  *maxRestarts,
		RestartDelay: *restartDelay,
		Heartbeat: client.HeartbeatOptions{
			Enabled:      *hbEnabled,
			Interval:     *hbInterval,
			Timeout:      *hbTimeout,
			Port:         firstNonEmpty(strings.TrimSpace(*hbPort), cfg.Server.Port),
			SocksAddress: firstNonEmpty(strings.TrimSpace(*hbSocks), cfg.Client.SocksAddress),
		},
	}

	if err := clientServiceRunFunc(ctx, opts); err != nil {
		logging.Error("xp2p client service failed", "err", err)
		return 1
	}
	return 0
}
