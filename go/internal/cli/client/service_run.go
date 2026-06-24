package clientcmd

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/service"
)

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

	logging.Configure(logging.Options{Output: logWriter, Level: os.Getenv(logging.EnvLogLevel)})

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
