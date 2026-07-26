package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/bootstrapapply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/service"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func runClientServiceCommon(ctx context.Context, opts ServiceOptions) error {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	configDirName := strings.TrimSpace(opts.ConfigDir)
	if configDirName == "" {
		configDirName = DefaultClientConfigDir
	}

	desiredConfigDir, err := config.DesiredExtensionsDirForRole(apply.RoleClient)
	if err != nil {
		return err
	}
	liveConfigDir, err := config.LiveRoleDir(apply.RoleClient)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(desiredConfigDir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	var diagnostics *server.BackgroundServer
	if port := strings.TrimSpace(opts.DiagPort); port != "" {
		diagnostics, err = server.StartBackground(ctx, server.Options{Port: port, InstallDir: installDir, LiveDir: liveConfigDir})
		if err != nil {
			logging.Warn("xp2p client diagnostics: failed to start responders", "port", port, "err", err)
		}
	}
	defer func() {
		if diagnostics != nil {
			stopCtx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			if err := diagnostics.Stop(stopCtx); err != nil {
				logging.Warn("xp2p client diagnostics shutdown failed", "err", err)
			}
		}
	}()

	baseRunOpts := RunOptions{
		InstallDir:        installDir,
		ConfigDir:         configDirName,
		Heartbeat:         opts.Heartbeat,
		TunEnabled:        opts.TunEnabled,
		TunName:           opts.TunName,
		TunMTU:            opts.TunMTU,
		TunAddr:           opts.TunAddr,
		TunMode:           opts.TunMode,
		DNSServers:        opts.DNSServers,
		FullTunnelVerbose: opts.FullTunnelVerbose,
		FullTunnelTag:     opts.FullTunnelTag,
	}

	stateRoot := installDir
	if runtime.GOOS == "windows" {
		stateRoot = config.ConfigRoot()
	}
	ignorePaths := []string{
		filepath.Join(stateRoot, layout.HeartbeatStateFileName),
		filepath.Join(stateRoot, layout.ClientHeartbeatStateFileName),
		filepath.Join(stateRoot, layout.ServerHeartbeatStateFileName),
	}

	runnerOpts := service.Options{
		Name:               "client",
		WatchFiles:         []string{config.ApplyRequestPath()},
		OnWatchFileChange:  handleClientApplyRequestChange,
		WatchDebounce:      400 * time.Millisecond,
		IgnorePaths:        ignorePaths,
		MaxRestarts:        opts.MaxRestarts,
		RestartDelay:       opts.RestartDelay,
		MaxWatchRestarts:   3,
		WatchRestartWindow: 30 * time.Second,
	}

	logWatcherStop, err := service.StartLogWatcher(ctx, service.LogWatchOptions{
		Name:          "client",
		Paths:         []string{desiredConfigDir},
		Files:         []string{filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))},
		IgnorePrefix:  []string{config.StateRoot()},
		WatchDebounce: 400 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := service.StopWithTimeout(logWatcherStop); err != nil {
			logging.Warn("xp2p client log watcher shutdown failed", "err", err)
		}
	}()

	desiredWatcherStop, err := service.StartConfigWatcher(ctx, service.ConfigWatchOptions{
		Paths:         []string{desiredConfigDir},
		Files:         []string{filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))},
		IgnorePrefix:  []string{config.StateRoot()},
		WatchDebounce: 400 * time.Millisecond,
		OnChange: func(_ context.Context, path string) {
			reqPath := config.ApplyRequestPath()
			errPath := config.ApplyErrorPath()
			if err := apply.RemoveRoleMarkers(reqPath, errPath, apply.RoleClient); err != nil {
				logging.Warn("xp2p client watcher: apply marker cleanup failed", "err", err)
			}
			req, err := apply.NewRequest(apply.RoleClient)
			if err != nil {
				logging.Warn("xp2p client watcher: apply request create failed", "err", err)
				return
			}
			if err := apply.WriteRequest(reqPath, req, config.AuditLogPath()); err != nil {
				logging.Warn("xp2p client watcher: apply request write failed", "err", err)
			}
		},
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := service.StopWithTimeout(desiredWatcherStop); err != nil {
			logging.Warn("xp2p client config watcher shutdown failed", "err", err)
		}
	}()

	if err := seedApplyRequestOnServiceStart(apply.RoleClient, liveConfigDir, desiredConfigDir); err != nil {
		return err
	}

	if err := service.Run(ctx, runnerOpts, func(runCtx context.Context) error {
		hasConfig, err := hasClientConfig(liveConfigDir)
		if err != nil {
			return err
		}
		if !hasConfig {
			logging.Info("xp2p client service: no config available; stopping",
				"config_dir", liveConfigDir,
				"xray", filepath.Join(liveConfigDir, layout.XrayConfigFileName),
			)
			return nil
		}
		return Run(runCtx, baseRunOpts)
	}); err != nil {
		if markerErr := writeApplyErrorForExistingRequest(apply.RoleClient, err); markerErr != nil {
			logging.Warn("xp2p client service: apply error write failed", "err", markerErr)
		}
		restoreFullTunnelOnStop(installDir, configDirName)
		return fmt.Errorf("xp2p client service: %w", err)
	}
	restoreFullTunnelOnStop(installDir, configDirName)
	return nil
}

func writeApplyErrorForExistingRequest(role string, reason error) error {
	if reason == nil {
		return nil
	}
	req, exists, err := apply.ReadRequestForRole(config.ApplyRequestPath(), role)
	if err != nil {
		return err
	}
	if !exists {
		req, err = apply.NewRequest(role)
		if err != nil {
			return err
		}
		if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
			return err
		}
	}
	if err := apply.WriteError(config.ApplyErrorPath(), apply.ErrorMarker{
		RequestID: req.ID,
		Role:      role,
		Reason:    reason.Error(),
	}, config.AuditLogPath()); err != nil {
		return err
	}
	return nil
}

func handleClientApplyRequestChange(ctx context.Context, path string) (service.WatchFileAction, error) {
	if filepath.Clean(path) != filepath.Clean(config.ApplyRequestPath()) {
		return service.WatchFileRestart, nil
	}
	result, err := tryRuntimeApplyPending(ctx, apply.RoleClient)
	if err != nil {
		return service.WatchFileRestart, err
	}
	switch result {
	case xraylive.RuntimeApplyApplied, xraylive.RuntimeApplyNoop, xraylive.RuntimeApplyFailed, xraylive.RuntimeApplySkipped:
		return service.WatchFileHandled, nil
	case xraylive.RuntimeApplyUnsupported, xraylive.RuntimeApplyServiceLayerRequired:
		return service.WatchFileRestart, nil
	default:
		return service.WatchFileRestart, nil
	}
}

func hasClientConfig(liveConfigDir string) (bool, error) {
	if _, err := os.Stat(config.ApplyRequestPath()); err == nil {
		return true, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat apply.request: %w", err)
	}

	required := []string{layout.XrayConfigFileName, layout.RuntimeMetaFileName}
	return configFilesPresent(liveConfigDir, required)
}

func seedApplyRequestOnServiceStart(role string, liveConfigDir string, desiredExtensionsDir string) error {
	seeded, err := bootstrapapply.Seed(role, liveConfigDir, desiredExtensionsDir)
	if seeded {
		logging.Info("xp2p client bootstrap: apply request recorded")
	}
	return err
}
