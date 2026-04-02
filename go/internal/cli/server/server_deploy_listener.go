package servercmd

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

type deployServer struct {
	ListenAddr string
	Expected   deploylink.EncryptedLink
	Once       bool
	Timeout    time.Duration
	Cfg        config.Config
}

type runSignal struct {
	ok         bool
	completed  bool
	status     string
	installDir string
	configDir  string
}

func (s *deployServer) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.ListenAddr)
	if err != nil {
		return err
	}
	defer ln.Close()

	results := make(chan runSignal, 4)

	idleTimer := time.NewTimer(s.Timeout)
	defer idleTimer.Stop()

	var (
		runCancel      context.CancelFunc
		diagCancel     context.CancelFunc
		runDoneCh      chan error
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
			if diagCancel != nil {
				diagCancel()
				diagCancel = nil
			}
			if runDoneCh != nil {
				select {
				case <-runDoneCh:
				case <-time.After(5 * time.Second):
					logging.Warn("xp2p server deploy: shutdown timed out waiting for xray-core")
				}
				runDoneCh = nil
			}
			return ctx.Err()
		case sig := <-results:
			switch {
			case sig.completed:
				logging.Info("xp2p server deploy: completion requested", "status", strings.TrimSpace(sig.status))
				if diagCancel != nil {
					diagCancel()
					diagCancel = nil
				}
				if runCancel != nil {
					runCancel()
					switchToTun = true
				} else if s.Once {
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
				if err := s.applyMode(sig.installDir, sig.configDir, false); err != nil {
					logging.Warn("xp2p server deploy: proxy mode setup failed", "err", err)
				}
				diagCtx, diagStop := context.WithCancel(ctx)
				if err := server.StartBackground(diagCtx, server.Options{
					Port:       s.Cfg.Server.Port,
					InstallDir: s.Cfg.Server.InstallDir,
				}); err != nil {
					logging.Warn("xp2p server deploy: diagnostics start failed", "err", err)
					diagStop()
				} else {
					diagCancel = diagStop
				}
				runCtx, runStop := context.WithCancel(ctx)
				runCancel = runStop
				runDone := make(chan error, 1)
				runDoneCh = runDone
				logging.Info("xp2p server deploy: starting xray-core", "install_dir", sig.installDir, "config_dir", sig.configDir)
				go func(installDir, configDir string) {
					runDone <- server.Run(runCtx, server.RunOptions{
						InstallDir: installDir,
						ConfigDir:  configDir,
						TunEnabled: false,
						TunName:    s.Cfg.Server.TunName,
						TunMTU:     s.Cfg.Server.TunMTU,
						TunAddr:    s.Cfg.Server.TunAddr,
					})
				}(sig.installDir, sig.configDir)
			}
		case err := <-runDoneCh:
			runDoneCh = nil
			runCancel = nil
			if diagCancel != nil {
				diagCancel()
				diagCancel = nil
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
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}

		go s.handleConn(ctx, conn, results)
	}
}

func (s *deployServer) applyMode(installDir, configDir string, tunEnabled bool) error {
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
	if err := s.applyMode(installDir, configDir, true); err != nil {
		logging.Warn("xp2p server deploy: tun mode setup failed", "err", err)
	}
	logging.Info("xp2p server deploy: service start skipped after deploy")
}

func logDeployPaths(message, updatedPath string) {
	applyPath := config.ApplyRequestPath()
	applyDir := filepath.Dir(applyPath)
	logging.Info(
		message,
		"mode_config", updatedPath,
		"live_config", config.ConfigPath(layout.ServerConfigFileName),
		"pending_config", config.PendingConfigPath(layout.ServerConfigFileName),
		"apply_dir", applyDir,
		"apply_request", applyPath,
		"live_exists", fileExists(config.ConfigPath(layout.ServerConfigFileName)),
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
