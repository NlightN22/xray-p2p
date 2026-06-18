package ping

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

// Options describes how Ping should behave.
type Options struct {
	Count      int
	Timeout    time.Duration
	Proto      string
	Port       int
	SocksProxy string
	Continuous bool
	KeepOpen   bool
	Reporter   Reporter
	Silent     bool
}

// Reporter is invoked when a TCP ping succeeds allowing callers to emit
// auxiliary payloads before the connection is closed.
type Reporter interface {
	Report(ctx context.Context, conn net.Conn, result Result) error
}

// Result captures statistics associated with a single ping request.
type Result struct {
	Seq    int
	Target string
	Proto  string
	RTT    time.Duration
}

const (
	defaultTimeout   = 3 * time.Second
	minCount         = 1
	protoTCP         = "tcp"
	protoUDP         = "udp"
	pingRequest      = "PING"
	keepOpenRequest  = "PING_KEEP_OPEN"
	expectedResponse = "PONG"
)

// Run performs application-level ping against the xp2p service.
func Run(ctx context.Context, target string, opts Options) error {
	if target == "" {
		return errors.New("ping target is required")
	}

	count := opts.Count
	if count < minCount {
		count = 4
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	proto := strings.ToLower(opts.Proto)
	if proto == "" {
		proto = protoTCP
	}
	if proto != protoTCP && proto != protoUDP {
		return fmt.Errorf("unsupported protocol %q", proto)
	}
	if opts.KeepOpen && proto != protoTCP {
		return errors.New("keep-open mode supports only tcp protocol")
	}

	port := opts.Port
	if port == 0 {
		p, err := strconv.Atoi(server.DefaultPort)
		if err != nil {
			return fmt.Errorf("invalid default port: %s", server.DefaultPort)
		}
		port = p
	}

	targetAddr := fmt.Sprintf("%s:%d", target, port)

	var sent, received int
	fields := []any{"target", targetAddr, "proto", proto}
	if opts.SocksProxy != "" {
		fields = append(fields, "socks_proxy", opts.SocksProxy)
	}
	logger := logging.With(fields...)
	logger.Debug("ping session started", "count", count, "timeout", timeout)
	if opts.KeepOpen {
		return runKeepOpen(ctx, targetAddr, timeout, opts)
	}

	for seq := 1; opts.Continuous || seq <= count; seq++ {
		select {
		case <-ctx.Done():
			if !opts.Silent {
				fmt.Println("interrupted")
			}
			if opts.Silent {
				logger.Debug("ping session interrupted", "sent", sent, "received", received)
			} else {
				logger.Info("ping session interrupted", "sent", sent, "received", received)
			}
			return ctx.Err()
		default:
		}

		var err error
		var rtt time.Duration
		switch proto {
		case protoTCP:
			logger.Debug("sending tcp ping", "seq", seq)
			rtt, err = pingTCP(ctx, targetAddr, timeout, opts.SocksProxy, seq, opts.Reporter)
		case protoUDP:
			if opts.SocksProxy != "" {
				logger.Warn("udp ping via socks proxy is not supported", "seq", seq)
				err = errors.New("UDP ping via SOCKS5 proxy is not supported yet (TODO: implement RFC 1928 UDP ASSOCIATE)")
				break
			}
			// TODO: support dokodemo or other proxy transports once available in diagnostics ping.
			logger.Debug("sending udp ping", "seq", seq)
			rtt, err = pingUDP(ctx, target, port, timeout)
		}

		sent++
		if err != nil {
			if !opts.Silent {
				fmt.Printf("Request %d failed: %v\n", seq, err)
			}
			if opts.Silent {
				logger.Debug("ping request failed", "seq", seq, "err", err)
			} else {
				logger.Warn("ping request failed", "seq", seq, "err", err)
			}
		} else {
			received++
			formatted := rtt.Round(time.Millisecond)
			if !opts.Silent {
				fmt.Printf("Reply from %s: seq=%d time=%s proto=%s\n",
					targetAddr, seq, formatted, proto)
			}
			logger.Debug("ping reply received", "seq", seq, "rtt", rtt)
		}

		if opts.Continuous || seq < count {
			select {
			case <-time.After(1 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if !opts.Silent {
		printSummary(sent, received)
	}
	if opts.Silent {
		logger.Debug("ping session completed", "sent", sent, "received", received)
	} else {
		logger.Info("ping session completed", "sent", sent, "received", received)
	}
	if received == 0 {
		return errors.New("no replies received")
	}
	return nil
}

func newNonce() (string, error) {
	raw := make([]byte, 10)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return enc.EncodeToString(raw), nil
}

func validateResponse(raw, nonce string) error {
	resp := strings.TrimSpace(raw)
	fields := strings.Fields(resp)
	if len(fields) == 0 || !strings.EqualFold(fields[0], expectedResponse) {
		return fmt.Errorf("unexpected response: %q", raw)
	}
	if len(fields) != 2 || !strings.EqualFold(fields[1], nonce) {
		return fmt.Errorf("unexpected response: %q", raw)
	}
	return nil
}

func printSummary(sent, received int) {
	lost := sent - received
	var lossPercent float64
	if sent > 0 {
		lossPercent = float64(lost) / float64(sent) * 100
	}
	fmt.Printf("\nPackets: sent = %d, received = %d, lost = %d (%.0f%% loss)\n",
		sent, received, lost, lossPercent)
}
