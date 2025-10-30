package ping

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/server"
)

// Options describes how Ping should behave.
type Options struct {
	Count   int
	Timeout time.Duration
	Proto   string
	Port    int
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

	for seq := 1; seq <= count; seq++ {
		select {
		case <-ctx.Done():
			fmt.Println("interrupted")
			return ctx.Err()
		default:
		}

		start := time.Now()
		var err error
		switch proto {
		case protoTCP:
			err = pingTCP(ctx, targetAddr, timeout)
		case protoUDP:
			err = pingUDP(ctx, target, port, timeout)
		}

		sent++
		if err != nil {
			fmt.Printf("Request %d failed: %v\n", seq, err)
		} else {
			received++
			fmt.Printf("Reply from %s: seq=%d time=%s proto=%s\n",
				targetAddr, seq, time.Since(start).Round(time.Millisecond), proto)
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

	printSummary(sent, received)
	if received == 0 {
		return errors.New("no replies received")
	}
	return nil
}

func pingTCP(ctx context.Context, addr string, timeout time.Duration) error {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}

	if _, err = conn.Write([]byte(pingRequestBody)); err != nil {
		return err
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}

	if !strings.EqualFold(strings.TrimSpace(string(buf[:n])), expectedResponse) {
		return fmt.Errorf("unexpected response: %q", string(buf[:n]))
	}

	return nil
}

func pingUDP(ctx context.Context, host string, port int, timeout time.Duration) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}

	if _, err = conn.Write([]byte(pingRequestBody)); err != nil {
		return err
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}

	if !strings.EqualFold(strings.TrimSpace(string(buf[:n])), expectedResponse) {
		return fmt.Errorf("unexpected response: %q", string(buf[:n]))
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
