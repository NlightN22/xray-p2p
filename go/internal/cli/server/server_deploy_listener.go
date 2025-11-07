package servercmd

import (
	"context"
	"net"
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
	defer close(results)

	idleTimer := time.NewTimer(s.Timeout)
	defer idleTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig := <-results:
			if sig.ok {
				if err := server.StartBackground(ctx, server.Options{Port: s.Cfg.Server.Port}); err != nil {
					logging.Warn("xp2p server deploy: diagnostics start failed", "err", err)
				}
				logging.Info("xp2p server deploy: starting xray-core", "install_dir", sig.installDir, "config_dir", sig.configDir)
				if err := server.Run(ctx, server.RunOptions{InstallDir: sig.installDir, ConfigDir: sig.configDir}); err != nil {
					logging.Error("xp2p server deploy: xray-core start failed", "err", err)
				}
				if s.Once {
					return nil
				}
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
