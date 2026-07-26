package servercmd

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

type deployServer struct {
	ListenAddr string
	Expected   deploylink.EncryptedLink
	Once       bool
	Timeout    time.Duration
	Cfg        config.Config
}

type runSignal struct {
	ok           bool
	completed    bool
	status       string
	installDir   string
	configDir    string
	runConfigDir string
	cleanupDir   string
	skipRun      bool
	applyHandled bool
}

func (s *deployServer) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.ListenAddr)
	if err != nil {
		return err
	}
	results := make(chan runSignal, 4)
	stopAccept := make(chan struct{})
	var acceptWG sync.WaitGroup
	acceptWG.Add(1)
	go func() {
		defer acceptWG.Done()
		select {
		case <-ctx.Done():
			_ = ln.Close()
		case <-stopAccept:
		}
	}()
	defer func() {
		close(stopAccept)
		acceptWG.Wait()
	}()
	handlerCtx, cancelHandlers := context.WithCancel(ctx)
	var handlers sync.WaitGroup
	var connections sync.Map
	defer func() {
		_ = ln.Close()
		cancelHandlers()
		connections.Range(func(key, _ any) bool {
			_ = key.(net.Conn).Close()
			return true
		})
		handlersDone := make(chan struct{})
		go func() {
			handlers.Wait()
			close(handlersDone)
		}()
		for {
			select {
			case <-results:
			case <-handlersDone:
				return
			}
		}
	}()

	idleTimer := time.NewTimer(s.Timeout)
	defer idleTimer.Stop()

	var (
		runCancel      context.CancelFunc
		diagnostics    *server.BackgroundServer
		runDoneCh      chan error
		runCleanupDir  string
		lastInstallDir string
		lastConfigDir  string
		switchToTun    bool
	)

	for {
		select {
		case <-ctx.Done():
			if runCancel != nil {
				runCancel()
			}
			if diagnostics != nil {
				stopDeployDiagnostics(diagnostics)
				diagnostics = nil
			}
			if runDoneCh != nil {
				select {
				case <-runDoneCh:
				case <-time.After(5 * time.Second):
					logging.Warn("xp2p server deploy: shutdown timed out waiting for xray-core")
				}
				runDoneCh = nil
			}
			if runCleanupDir != "" {
				_ = os.RemoveAll(runCleanupDir)
				runCleanupDir = ""
			}
			return ctx.Err()
		case sig := <-results:
			switch {
			case sig.completed:
				logging.Info("xp2p server deploy: completion requested", "status", strings.TrimSpace(sig.status))
				if diagnostics != nil {
					stopDeployDiagnostics(diagnostics)
					diagnostics = nil
				}
				if s.Once && runCancel == nil && sig.applyHandled {
					return nil
				}
				if runCancel != nil {
					runCancel()
					switchToTun = !sig.applyHandled
				} else if s.Once && !sig.applyHandled {
					s.applyTunAndStartService(ctx, lastInstallDir, lastConfigDir)
					return nil
				}
				if runCancel == nil && switchToTun {
					s.applyTunAndStartService(ctx, lastInstallDir, lastConfigDir)
					switchToTun = false
					if s.Once {
						return nil
					}
				}
			case sig.ok:
				if runDoneCh != nil {
					logging.Warn("xp2p server deploy: xray-core already running; ignoring duplicate request")
					continue
				}
				if sig.installDir != "" {
					lastInstallDir = sig.installDir
				}
				if sig.configDir != "" {
					lastConfigDir = sig.configDir
				}
				if sig.skipRun {
					continue
				}
				runCleanupDir = strings.TrimSpace(sig.cleanupDir)
				if err := s.applyMode(sig.installDir, sig.configDir, false); err != nil {
					logging.Warn("xp2p server deploy: proxy mode setup failed", "err", err)
				}
				owner, err := server.StartBackground(ctx, server.Options{
					Port:       s.Cfg.Server.Port,
					InstallDir: s.Cfg.Server.InstallDir,
					LiveDir:    sig.runConfigDir,
				})
				if err != nil {
					logging.Warn("xp2p server deploy: diagnostics start failed", "err", err)
				} else {
					diagnostics = owner
				}
				runCtx, runStop := context.WithCancel(ctx)
				runCancel = runStop
				runDone := make(chan error, 1)
				runDoneCh = runDone
				logging.Info("xp2p server deploy: starting xray-core", "install_dir", sig.installDir, "config_dir", sig.runConfigDir)
				go func(installDir, configDir string) {
					runDone <- server.RunDeploy(runCtx, server.DeployRunOptions{
						InstallDir: installDir,
						ConfigDir:  configDir,
					})
				}(sig.installDir, sig.runConfigDir)
			}
		case err := <-runDoneCh:
			runDoneCh = nil
			runCancel = nil
			if diagnostics != nil {
				stopDeployDiagnostics(diagnostics)
				diagnostics = nil
			}
			if runCleanupDir != "" {
				_ = os.RemoveAll(runCleanupDir)
				runCleanupDir = ""
			}
			if switchToTun {
				s.applyTunAndStartService(ctx, lastInstallDir, lastConfigDir)
				switchToTun = false
				if s.Once {
					return nil
				}
			}
			if err != nil {
				logging.Error("xp2p server deploy: xray-core start failed", "err", err)
			}
			if s.Once {
				return nil
			}
		default:
		}

		if s.Timeout > 0 {
			select {
			case <-idleTimer.C:
				logging.Info("xp2p server deploy: idle timeout reached; shutting down")
				return nil
			default:
			}
		}

		if tcpLn, ok := ln.(*net.TCPListener); ok {
			_ = tcpLn.SetDeadline(time.Now().Add(time.Second))
		}
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}

		handlers.Add(1)
		connections.Store(conn, struct{}{})
		go func() {
			defer handlers.Done()
			defer connections.Delete(conn)
			s.handleConn(handlerCtx, conn, results)
		}()
	}
}

func stopDeployDiagnostics(owner *server.BackgroundServer) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := owner.Stop(ctx); err != nil {
		logging.Warn("xp2p server deploy: diagnostics shutdown failed", "err", err)
	}
}

func (s *deployServer) applyMode(installDir, configDir string, tunEnabled bool) error {
	pendingPath := filepath.Clean(config.PendingConfigPath(layout.ServerConfigFileName))
	if _, err := server.EnsureServerXrayConfig(pendingPath); err != nil {
		return err
	}
	updatedPath, err := config.UpdateTunEnabled("", "server", tunEnabled)
	if err != nil {
		return err
	}
	if _, err := config.EnsureTunSettings("", "server", tunEnabled, s.Cfg.Server.TunName, s.Cfg.Server.TunMTU, s.Cfg.Server.TunAddr); err != nil {
		return err
	}
	logDeployPaths("xp2p server deploy: mode config updated", updatedPath)
	req, err := apply.NewRequest(apply.RoleServer)
	if err != nil {
		return err
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		return err
	}
	logDeployPaths("xp2p server deploy: apply request written", updatedPath)
	return nil
}

func (s *deployServer) applyTunAndStartService(ctx context.Context, installDir, configDir string) {
	if !s.Cfg.Server.TunEnabled {
		logging.Info("xp2p server deploy: tun mode skipped; server tun is disabled")
		logServerServiceApplyHint(ctx)
		return
	}
	if err := s.applyMode(installDir, configDir, true); err != nil {
		logging.Warn("xp2p server deploy: tun mode setup failed", "err", err)
	}
	logServerServiceApplyHint(ctx)
}

func logServerServiceApplyHint(ctx context.Context) {
	ctrl := servicecontrol.Default()
	statusCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status, err := ctrl.Status(statusCtx, servicecontrol.RoleServer)
	if err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Warn("xp2p server deploy: service manager unavailable; start or restart service to apply pending changes")
			return
		}
		logging.Warn("xp2p server deploy: service status check failed", "err", err)
		return
	}
	if status.Active {
		logging.Info("xp2p server deploy: service active; restart required to apply pending changes")
	} else {
		logging.Info("xp2p server deploy: service inactive; start required to apply pending changes")
	}
}

func logDeployPaths(message, updatedPath string) {
	applyPath := config.ApplyRequestPath()
	applyDir := filepath.Dir(applyPath)
	logging.Info(
		message,
		"mode_config", updatedPath,
		"live_config", config.LiveConfigPath(layout.ServerConfigFileName),
		"pending_config", config.PendingConfigPath(layout.ServerConfigFileName),
		"apply_dir", applyDir,
		"apply_request", applyPath,
		"live_exists", fileExists(config.LiveConfigPath(layout.ServerConfigFileName)),
		"pending_exists", fileExists(config.PendingConfigPath(layout.ServerConfigFileName)),
		"apply_dir_exists", dirExists(applyDir),
		"apply_request_exists", fileExists(applyPath),
	)
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
