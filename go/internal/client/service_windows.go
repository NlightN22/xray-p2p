//go:build windows

package client

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/svc"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/service"
)

// RunService launches the managed client service loop on Windows.
func RunService(ctx context.Context, opts ServiceOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if runningAsWindowsService() {
		handler := &clientWindowsService{opts: opts}
		if err := svc.Run("xp2p-client", handler); err != nil {
			return fmt.Errorf("xp2p client windows service: %w", err)
		}
		return handler.err
	}
	return runClientServiceLoop(ctx, opts)
}

func runClientServiceLoop(ctx context.Context, opts ServiceOptions) error {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	configDirName := strings.TrimSpace(opts.ConfigDir)
	if configDirName == "" {
		configDirName = DefaultClientConfigDir
	}

	configDirPath, err := resolveConfigDir(installDir, configDirName)
	if err != nil {
		return err
	}

	var diagCancel context.CancelFunc
	if port := strings.TrimSpace(opts.DiagPort); port != "" {
		bgCtx, cancel := context.WithCancel(ctx)
		if err := server.StartBackground(bgCtx, server.Options{Port: port, InstallDir: installDir}); err != nil {
			cancel()
			logging.Warn("xp2p client diagnostics: failed to start responders", "port", port, "err", err)
		} else {
			diagCancel = cancel
		}
	}
	defer func() {
		if diagCancel != nil {
			diagCancel()
		}
	}()

	runOpts := RunOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		ErrorLogPath: strings.TrimSpace(opts.XrayLogPath),
		Heartbeat:    opts.Heartbeat,
	}

	watchPaths := []string{
		installDir,
		configDirPath,
	}
	ignorePaths := []string{
		filepath.Join(installDir, layout.HeartbeatStateFileName),
	}

	runnerOpts := service.Options{
		Name:         "client",
		WatchPaths:   watchPaths,
		IgnorePaths:  ignorePaths,
		MaxRestarts:  opts.MaxRestarts,
		RestartDelay: opts.RestartDelay,
	}

	if err := service.Run(ctx, runnerOpts, func(runCtx context.Context) error {
		return Run(runCtx, runOpts)
	}); err != nil {
		return fmt.Errorf("xp2p client service: %w", err)
	}
	return nil
}

type clientWindowsService struct {
	opts ServiceOptions
	err  error
}

func (s *clientWindowsService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepts = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- runClientServiceLoop(ctx, s.opts)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: accepts}

	for {
		select {
		case err := <-errCh:
			s.err = err
			if err != nil {
				logging.Error("xp2p client service failed", "err", err)
				changes <- svc.Status{State: svc.Stopped}
				return false, 1
			}
			changes <- svc.Status{State: svc.Stopped}
			return false, 0
		case change := <-r:
			switch change.Cmd {
			case svc.Interrogate:
				changes <- change.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending, Accepts: accepts}
				cancel()
			default:
			}
		}
	}
}

func runningAsWindowsService() bool {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return isService
}
