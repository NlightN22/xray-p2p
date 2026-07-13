package server

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
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/service"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func runServerServiceCommon(ctx context.Context, opts ServiceOptions) error {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	configDirName := strings.TrimSpace(opts.ConfigDir)
	if configDirName == "" {
		configDirName = DefaultServerConfigDir
	}

	desiredConfigDir, err := config.DesiredExtensionsDirForRole(apply.RoleServer)
	if err != nil {
		return err
	}
	liveConfigDir, err := config.LiveRoleDir(apply.RoleServer)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(desiredConfigDir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	recordServerBootstrapApplyError(StageLegacyCredentialRotation(ctx))
	cfg, err := config.Load(config.Options{Path: config.ConfigPath(layout.ServerConfigFileName), AllowInvalid: true})
	if err != nil {
		return err
	}
	stopIdentitySync := startIdentitySyncScheduler(ctx, cfg)
	defer stopIdentitySync()

	var diagCancel context.CancelFunc
	if port := strings.TrimSpace(opts.DiagPort); port != "" {
		bgCtx, cancel := context.WithCancel(ctx)
		if err := StartBackground(bgCtx, Options{Port: port, InstallDir: installDir, LiveDir: liveConfigDir}); err != nil {
			cancel()
			logging.Warn("xp2p server diagnostics: failed to start responders", "port", port, "err", err)
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
		InstallDir: installDir,
		ConfigDir:  configDirName,
		TunEnabled: opts.TunEnabled,
		TunName:    opts.TunName,
		TunMTU:     opts.TunMTU,
		TunAddr:    opts.TunAddr,
	}

	stateRoot := installDir
	if runtime.GOOS == "windows" {
		stateRoot = config.ConfigRoot()
	}
	ignorePaths := []string{
		filepath.Join(stateRoot, layout.ClientHeartbeatStateFileName),
		filepath.Join(stateRoot, layout.HeartbeatStateFileName),
		filepath.Join(stateRoot, layout.ServerHeartbeatStateFileName),
	}

	runnerOpts := service.Options{
		Name:               "server",
		WatchFiles:         []string{config.ApplyRequestPath()},
		OnWatchFileChange:  handleServerApplyRequestChange,
		WatchDebounce:      400 * time.Millisecond,
		IgnorePaths:        ignorePaths,
		MaxRestarts:        opts.MaxRestarts,
		RestartDelay:       opts.RestartDelay,
		MaxWatchRestarts:   3,
		WatchRestartWindow: 30 * time.Second,
	}

	logWatcherStop, err := service.StartLogWatcher(ctx, service.LogWatchOptions{
		Name:          "server",
		Paths:         []string{desiredConfigDir},
		Files:         []string{filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))},
		IgnorePrefix:  []string{config.StateRoot()},
		WatchDebounce: 400 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer logWatcherStop()

	desiredWatcherStop, err := service.StartConfigWatcher(ctx, service.ConfigWatchOptions{
		Paths:         []string{desiredConfigDir},
		Files:         []string{filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))},
		IgnorePrefix:  []string{config.StateRoot()},
		WatchDebounce: 400 * time.Millisecond,
		OnChange: func(path string) {
			reqPath := config.ApplyRequestPath()
			errPath := config.ApplyErrorPath()
			if err := apply.RemoveRoleMarkers(reqPath, errPath, apply.RoleServer); err != nil {
				logging.Warn("xp2p server watcher: apply marker cleanup failed", "err", err)
			}
			req, err := apply.NewRequest(apply.RoleServer)
			if err != nil {
				logging.Warn("xp2p server watcher: apply request create failed", "err", err)
				return
			}
			if err := apply.WriteRequest(reqPath, req, config.AuditLogPath()); err != nil {
				logging.Warn("xp2p server watcher: apply request write failed", "err", err)
			}
		},
	})
	if err != nil {
		return err
	}
	defer desiredWatcherStop()

	if err := seedApplyRequestOnServiceStart(apply.RoleServer, liveConfigDir, desiredConfigDir); err != nil {
		return err
	}

	if err := service.Run(ctx, runnerOpts, func(runCtx context.Context) error {
		hasConfig, err := hasServerConfig(liveConfigDir)
		if err != nil {
			return err
		}
		if !hasConfig {
			logging.Info("xp2p server service: no config available; stopping",
				"config_dir", liveConfigDir,
				"xray", filepath.Join(liveConfigDir, layout.XrayConfigFileName),
			)
			return nil
		}
		return Run(runCtx, runOpts)
	}); err != nil {
		if markerErr := writeApplyErrorForExistingRequest(apply.RoleServer, err); markerErr != nil {
			logging.Warn("xp2p server service: apply error write failed", "err", markerErr)
		}
		return fmt.Errorf("xp2p server service: %w", err)
	}
	return nil
}

func writeApplyErrorForExistingRequest(role string, reason error) error {
	if reason == nil {
		return nil
	}
	req, exists, err := apply.ReadRequest(config.ApplyRequestPath())
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
	} else if !req.MatchesRole(role) {
		return nil
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

func recordServerBootstrapApplyError(err error) {
	if err == nil {
		return
	}
	if markerErr := writeApplyErrorForExistingRequest(apply.RoleServer, err); markerErr != nil {
		logging.Warn("xp2p server bootstrap: apply error write failed", "err", markerErr)
		return
	}
	logging.Warn("xp2p server bootstrap: legacy credential rotation skipped", "err", err)
}

func handleServerApplyRequestChange(ctx context.Context, path string) (service.WatchFileAction, error) {
	if filepath.Clean(path) != filepath.Clean(config.ApplyRequestPath()) {
		return service.WatchFileRestart, nil
	}
	result, err := tryRuntimeApplyPending(ctx, apply.RoleServer)
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

func hasServerConfig(liveConfigDir string) (bool, error) {
	if _, err := os.Stat(config.ApplyRequestPath()); err == nil {
		return true, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat apply.request: %w", err)
	}

	required := []string{layout.XrayConfigFileName, layout.RuntimeMetaFileName}
	return configFilesPresent(liveConfigDir, required)
}

func seedApplyRequestOnServiceStart(role string, liveConfigDir string, desiredExtensionsDir string) error {
	if _, err := os.Stat(config.ApplyRequestPath()); err == nil {
		return nil
	}

	desiredConfigPath, err := config.DesiredConfigPathForRole(role)
	if err != nil {
		return err
	}
	desiredInfo, err := os.Stat(desiredConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat desired config %s: %w", desiredConfigPath, err)
	}
	desiredLatest := desiredInfo.ModTime()

	if entries, err := os.ReadDir(desiredExtensionsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr != nil {
				continue
			}
			if info.ModTime().After(desiredLatest) {
				desiredLatest = info.ModTime()
			}
		}
	}

	liveMetaPath := filepath.Join(filepath.Clean(liveConfigDir), layout.RuntimeMetaFileName)
	liveMetaInfo, err := os.Stat(liveMetaPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat runtime metadata %s: %w", liveMetaPath, err)
		}
	} else if !desiredLatest.After(liveMetaInfo.ModTime()) {
		return nil
	}

	if err := apply.RemoveError(config.ApplyErrorPath()); err != nil {
		logging.Warn("xp2p server bootstrap: apply error cleanup failed", "err", err)
	}
	req, err := apply.NewRequest(role)
	if err != nil {
		return err
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		return err
	}
	logging.Info("xp2p server bootstrap: apply request recorded")
	return nil
}
