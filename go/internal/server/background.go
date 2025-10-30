package server

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const (
	// DefaultPort is the well known port used by xp2p helper services.
	DefaultPort  = "62022"
	pingRequest  = "PING"
	pingResponse = "PONG"
)

// StartBackground launches lightweight TCP and UDP responders that can be used
// by diagnostics routines. Listeners are shut down automatically when the
// supplied context is cancelled.
func StartBackground(ctx context.Context) error {
	var (
		once    sync.Once
		tcpLn   net.Listener
		udpConn net.PacketConn
		started bool
	)

	shutdown := func() {
		once.Do(func() {
			if tcpLn != nil {
				_ = tcpLn.Close()
			}
			if udpConn != nil {
				_ = udpConn.Close()
			}
		})
	}

	if ln, err := net.Listen("tcp", ":"+DefaultPort); err != nil {
		logging.Warn("unable to start TCP listener", "port", DefaultPort, "err", err)
	} else {
		tcpLn = ln
		started = true
		go func() {
			defer tcpLn.Close()
			for {
				conn, err := ln.Accept()
				if err != nil {
					select {
					case <-ctx.Done():
						return
					default:
						logging.Warn("tcp accept error", "err", err)
						continue
					}
				}
				go handleTCP(ctx, conn)
			}
		}()
	}

	if pc, err := net.ListenPacket("udp", ":"+DefaultPort); err != nil {
		logging.Warn("unable to start UDP listener", "port", DefaultPort, "err", err)
	} else {
		udpConn = pc
		started = true
		go handleUDP(ctx, udpConn)
	}

	if !started {
		return errors.New("xp2p: unable to bind TCP/UDP listeners")
	}

	go func() {
		<-ctx.Done()
		shutdown()
	}()

	return nil
}

func handleTCP(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(deadlineFromContext(ctx))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	if strings.EqualFold(strings.TrimSpace(line), pingRequest) {
		logging.Info("tcp ping received", "remote_addr", conn.RemoteAddr().String())
		_, _ = conn.Write([]byte(pingResponse + "\n"))
	}
}

func handleUDP(ctx context.Context, conn net.PacketConn) {
	defer conn.Close()
	buf := make([]byte, 1024)
	for {
		_ = conn.SetReadDeadline(deadlineFromContext(ctx))
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				logging.Warn("udp read error", "err", err)
				continue
			}
		}

		msg := strings.TrimSpace(string(buf[:n]))
		if strings.EqualFold(msg, pingRequest) {
			logging.Info("udp ping received", "remote_addr", addr.String())
			_, _ = conn.WriteTo([]byte(pingResponse+"\n"), addr)
		}
	}
}

func deadlineFromContext(ctx context.Context) time.Time {
	if dl, ok := ctx.Deadline(); ok {
		return dl
	}
	return time.Time{}
}
