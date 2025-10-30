package server

import (
	"context"
	"log"
	"net"
	"sync"
	"time"
)

const (
	// DefaultPort is the well known port used by xp2p helper services.
	DefaultPort = "62022"
)

// StartBackground launches lightweight TCP and UDP responders that can be used
// by diagnostics routines. Listeners are shut down automatically when the
// supplied context is cancelled.
func StartBackground(ctx context.Context) {
	var once sync.Once
	shutdown := func(l net.Listener, pc net.PacketConn) {
		once.Do(func() {
			if l != nil {
				_ = l.Close()
			}
			if pc != nil {
				_ = pc.Close()
			}
		})
	}

	tcpLn, err := net.Listen("tcp", ":"+DefaultPort)
	if err != nil {
		log.Printf("xp2p: warning: unable to start TCP listener on %s: %v", DefaultPort, err)
	} else {
		go func() {
			defer tcpLn.Close()
			for {
				conn, err := tcpLn.Accept()
				if err != nil {
					select {
					case <-ctx.Done():
						return
					default:
						log.Printf("xp2p: tcp accept error: %v", err)
						continue
					}
				}
				go handleTCP(ctx, conn)
			}
		}()
	}

	udpConn, err := net.ListenPacket("udp", ":"+DefaultPort)
	if err != nil {
		log.Printf("xp2p: warning: unable to start UDP listener on %s: %v", DefaultPort, err)
	} else {
		go handleUDP(ctx, udpConn)
	}

	go func() {
		<-ctx.Done()
		shutdown(tcpLn, udpConn)
	}()
}

func handleTCP(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(deadlineFromContext(ctx))
	_, _ = conn.Write([]byte("xp2p\n"))
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
				log.Printf("xp2p: udp read error: %v", err)
				continue
			}
		}
		if n > 0 {
			_, _ = conn.WriteTo([]byte("xp2p\n"), addr)
		}
	}
}

func deadlineFromContext(ctx context.Context) time.Time {
	if dl, ok := ctx.Deadline(); ok {
		return dl
	}
	return time.Time{}
}
