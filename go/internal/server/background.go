package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const (
	// DefaultPort is the well known port used by xp2p helper services.
	DefaultPort  = "62022"
	pingRequest  = "PING"
	pingResponse = "PONG"
)

const heartbeatPayloadTimeout = 250 * time.Millisecond

// Options controls background server behaviour.
type Options struct {
	Port       string
	InstallDir string
}

// StartBackground launches lightweight TCP and UDP responders that can be used
// by diagnostics routines. Listeners are shut down automatically when the
// supplied context is cancelled.
func StartBackground(ctx context.Context, opts Options) error {
	var (
		once     sync.Once
		tcpLn    net.Listener
		udpConn  net.PacketConn
		started  bool
		hbStore  *heartbeat.Store
		storeErr error
	)

	port := strings.TrimSpace(opts.Port)
	if port == "" {
		port = DefaultPort
	}

	storePath := ""
	if dir := strings.TrimSpace(opts.InstallDir); dir != "" {
		storePath = filepath.Join(dir, layout.HeartbeatStateFileName)
	}
	hbStore, storeErr = heartbeat.NewStore(storePath)
	if storeErr != nil {
		logging.Warn("heartbeat store disabled", "err", storeErr)
		hbStore, _ = heartbeat.NewStore("")
	}

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

	if ln, err := net.Listen("tcp", ":"+port); err != nil {
		logging.Warn("unable to start TCP listener", "port", port, "err", err)
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
				go handleTCP(ctx, conn, hbStore)
			}
		}()
	}

	if pc, err := net.ListenPacket("udp", ":"+port); err != nil {
		logging.Warn("unable to start UDP listener", "port", port, "err", err)
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

func handleTCP(ctx context.Context, conn net.Conn, store *heartbeat.Store) {
	defer conn.Close()
	_ = conn.SetDeadline(deadlineFromContext(ctx))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	if strings.EqualFold(strings.TrimSpace(line), pingRequest) {
		_, _ = conn.Write([]byte(pingResponse + "\n"))
		hadHeartbeat := consumeHeartbeatPayload(ctx, reader, conn, store)
		if !hadHeartbeat {
			logging.Info("tcp ping received", "remote_addr", conn.RemoteAddr().String())
		} else {
			logging.Debug("tcp heartbeat received", "remote_addr", conn.RemoteAddr().String())
		}
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

func consumeHeartbeatPayload(ctx context.Context, reader *bufio.Reader, conn net.Conn, store *heartbeat.Store) bool {
	if store == nil {
		return false
	}
	deadline := time.Now().Add(heartbeatPayloadTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetReadDeadline(deadline)
	line, err := reader.ReadString('\n')
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return false
		}
		if err == io.EOF && len(line) == 0 {
			return false
		}
		if len(line) == 0 {
			return false
		}
	}

	payloadRaw := strings.TrimSpace(line)
	if payloadRaw == "" {
		return false
	}

	var payload heartbeat.Payload
	if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
		logging.Warn("invalid heartbeat payload", "remote_addr", conn.RemoteAddr().String(), "err", err)
		return true
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now().UTC()
	}
	if _, err := store.Update(payload); err != nil {
		logging.Warn("unable to persist heartbeat", "tag", payload.Tag, "err", err)
		return true
	}
	logging.Debug("heartbeat recorded", "tag", payload.Tag, "host", payload.Host, "client_ip", payload.ClientIP)
	return true
}
