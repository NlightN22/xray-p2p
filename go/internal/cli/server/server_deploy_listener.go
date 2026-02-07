package servercmd

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
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
		runCancel  context.CancelFunc
		diagCancel context.CancelFunc
		runDoneCh  chan error
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
				} else if s.Once {
					return nil
				}
			case sig.ok:
				if runDoneCh != nil {
					logging.Warn("xp2p server deploy: xray-core already running; ignoring duplicate request")
					continue
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
						TunEnabled: s.Cfg.Server.TunEnabled,
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
