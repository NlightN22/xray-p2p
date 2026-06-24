package servercmd

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/service"
)

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
	logging.Configure(logging.Options{Output: logWriter, Level: os.Getenv(logging.EnvLogLevel)})

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
