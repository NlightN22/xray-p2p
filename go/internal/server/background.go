package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
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
	ListenAddr string
	Proto      string
	Quiet      bool
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
	listenAddr := strings.TrimSpace(opts.ListenAddr)
	if listenAddr == "" {
		listenAddr = ":" + port
	}

	proto := strings.ToLower(strings.TrimSpace(opts.Proto))
	switch proto {
	case "":
		proto = "both"
	case "tcp", "udp", "both":
	default:
		return fmt.Errorf("unsupported diagnostics protocol %q", opts.Proto)
	}

	storePath := ""
	storeRoot := strings.TrimSpace(opts.InstallDir)
	if runtime.GOOS == "windows" {
		storeRoot = config.ConfigRoot()
	}
	if storeRoot != "" {
		storePath = filepath.Join(storeRoot, layout.ServerHeartbeatStateFileName)
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

	if proto != "udp" {
		if ln, err := net.Listen("tcp", listenAddr); err != nil {
			logging.Warn("unable to start TCP listener", "addr", listenAddr, "err", err)
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
					go handleTCP(ctx, conn, hbStore, opts.Quiet)
				}
			}()
		}
	} else {
		tcpLn = nil
	}

	if proto != "tcp" {
		if pc, err := net.ListenPacket("udp", listenAddr); err != nil {
			logging.Warn("unable to start UDP listener", "addr", listenAddr, "err", err)
		} else {
			udpConn = pc
			started = true
			go handleUDP(ctx, udpConn, opts.Quiet)
		}
	}

	if !started {
		return errors.New("unable to bind diagnostics listeners")
	}

	go func() {
		<-ctx.Done()
		shutdown()
	}()

	return nil
}

func handleTCP(ctx context.Context, conn net.Conn, store *heartbeat.Store, quiet bool) {
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
		if !quiet {
			if !hadHeartbeat {
				logging.Info("tcp ping received", "remote_addr", conn.RemoteAddr().String())
			} else {
				logging.Debug("tcp heartbeat received", "remote_addr", conn.RemoteAddr().String())
			}
		}
	}
}

func handleUDP(ctx context.Context, conn net.PacketConn, quiet bool) {
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
			if !quiet {
				logging.Info("udp ping received", "remote_addr", addr.String())
			}
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
	payload.Timestamp = time.Time{}
	if _, err := store.Update(payload); err != nil {
		logging.Warn("unable to persist heartbeat", "tag", payload.Tag, "err", err)
		return true
	}
	logging.Debug("heartbeat recorded", "tag", payload.Tag, "host", payload.Host, "client_ip", payload.ClientIP)
	return true
}
