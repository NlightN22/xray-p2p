package servercmd

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
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
		runCancel  context.CancelFunc
		diagCancel context.CancelFunc
		runDoneCh  chan error
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
	modeLabel := "proxy"
	if tunEnabled {
		modeLabel = "tun"
	}
	updatedPath, err := config.UpdateTunEnabled("", "server", tunEnabled)
	if err != nil {
		return err
	}
	logging.Info("xp2p server deploy: mode config updated", "mode", modeLabel, "config", updatedPath)
	err = server.ApplyMode(server.ModeOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		TunEnabled: tunEnabled,
		TunName:    s.Cfg.Server.TunName,
		TunMTU:     s.Cfg.Server.TunMTU,
		TunAddr:    s.Cfg.Server.TunAddr,
	})
	if err == nil {
		return nil
	}
	if isPermissionError(err) {
		logging.Warn("xp2p server deploy: mode apply skipped due to permissions", "mode", modeLabel, "err", err)
		return nil
	}
	return err
}

func (s *deployServer) applyTunAndStartService(ctx context.Context, installDir, configDir string) {
	if err := s.applyMode(installDir, configDir, true); err != nil {
		logging.Warn("xp2p server deploy: tun mode setup failed", "err", err)
	}
	ctrl := servicecontrol.Default()
	if err := ctrl.Start(ctx, servicecontrol.RoleServer); err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Warn("xp2p server deploy: service start is not supported on this platform")
			return
		}
		logging.Warn("xp2p server deploy: server service start failed", "err", err)
		return
	}
	logging.Info("xp2p server deploy: server service started")
}

func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "operation not permitted") || strings.Contains(lower, "permission denied")
}
