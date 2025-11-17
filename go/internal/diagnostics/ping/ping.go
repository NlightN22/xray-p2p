package ping

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"golang.org/x/net/proxy"
)

// Options describes how Ping should behave.
type Options struct {
	Count      int
	Timeout    time.Duration
	Proto      string
	Port       int
	SocksProxy string
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
	pingRequestBody  = "PING\n"
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

	for seq := 1; seq <= count; seq++ {
		select {
		case <-ctx.Done():
			if !opts.Silent {
				fmt.Println("interrupted")
			}
			logger.Info("ping session interrupted", "sent", sent, "received", received)
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
			logger.Warn("ping request failed", "seq", seq, "err", err)
		} else {
			received++
			formatted := rtt.Round(time.Millisecond)
			if !opts.Silent {
				fmt.Printf("Reply from %s: seq=%d time=%s proto=%s\n",
					targetAddr, seq, formatted, proto)
			}
			logger.Debug("ping reply received", "seq", seq, "rtt", rtt)
		}

		// Simple pacing between requests.
		if seq < count {
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
	logger.Info("ping session completed", "sent", sent, "received", received)
	if received == 0 {
		return errors.New("no replies received")
	}
	return nil
}

func pingTCP(ctx context.Context, addr string, timeout time.Duration, socksProxy string, seq int, reporter Reporter) (time.Duration, error) {
	var (
		conn net.Conn
		err  error
	)
	if socksProxy == "" {
		dialer := &net.Dialer{Timeout: timeout}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialViaSocks(ctx, addr, socksProxy, timeout)
	}
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	if err := setDeadline(conn, timeout); err != nil {
		return 0, err
	}

	start := time.Now()
	if _, err = conn.Write([]byte(pingRequestBody)); err != nil {
		return 0, err
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return 0, err
	}

	if !strings.EqualFold(strings.TrimSpace(string(buf[:n])), expectedResponse) {
		return 0, fmt.Errorf("unexpected response: %q", string(buf[:n]))
	}

	rtt := time.Since(start)
	if reporter != nil {
		result := Result{
			Seq:    seq,
			Target: addr,
			Proto:  protoTCP,
			RTT:    rtt,
		}
		if err := reporter.Report(ctx, conn, result); err != nil {
			return rtt, err
		}
	}

	return rtt, nil
}

func pingUDP(ctx context.Context, host string, port int, timeout time.Duration) (time.Duration, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return 0, err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	if err := setDeadline(conn, timeout); err != nil {
		return 0, err
	}

	start := time.Now()
	if _, err = conn.Write([]byte(pingRequestBody)); err != nil {
		return 0, err
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return 0, err
	}

	if !strings.EqualFold(strings.TrimSpace(string(buf[:n])), expectedResponse) {
		return 0, fmt.Errorf("unexpected response: %q", string(buf[:n]))
	}

	return time.Since(start), nil
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

func dialViaSocks(ctx context.Context, addr, proxyAddr string, timeout time.Duration) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	base := &net.Dialer{Timeout: timeout}
	if deadline, ok := ctx.Deadline(); ok {
		base.Deadline = deadline
	}

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, base)
	if err != nil {
		return nil, fmt.Errorf("prepare SOCKS5 dialer %s: %w", proxyAddr, err)
	}

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect through SOCKS5 proxy %s: %w", proxyAddr, err)
	}

	return conn, nil
}

func setDeadline(conn net.Conn, timeout time.Duration) error {
	return conn.SetDeadline(time.Now().Add(timeout))
}
