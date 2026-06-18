package ping

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/net/proxy"
)

func pingTCP(ctx context.Context, addr string, timeout time.Duration, socksProxy string, seq int, reporter Reporter) (time.Duration, error) {
	conn, err := dialTCP(ctx, addr, socksProxy, timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	return exchangeTCP(ctx, conn, addr, timeout, seq, reporter)
}

func pingUDP(_ context.Context, host string, port int, timeout time.Duration) (time.Duration, error) {
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

	nonce, err := newNonce()
	if err != nil {
		return 0, err
	}
	request := pingRequest + " " + nonce + "\n"

	start := time.Now()
	if _, err = conn.Write([]byte(request)); err != nil {
		return 0, err
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return 0, err
	}

	if err := validateResponse(string(buf[:n]), nonce); err != nil {
		return 0, err
	}

	return time.Since(start), nil
}

func exchangeTCP(ctx context.Context, conn net.Conn, addr string, timeout time.Duration, seq int, reporter Reporter) (time.Duration, error) {
	return exchangeTCPRequest(ctx, conn, addr, pingRequest, timeout, seq, reporter)
}

func exchangeTCPRequest(ctx context.Context, conn net.Conn, addr string, requestName string, timeout time.Duration, seq int, reporter Reporter) (time.Duration, error) {
	if err := setDeadline(conn, timeout); err != nil {
		return 0, err
	}

	nonce, err := newNonce()
	if err != nil {
		return 0, err
	}
	request := requestName + " " + nonce + "\n"

	start := time.Now()
	if _, err = conn.Write([]byte(request)); err != nil {
		return 0, err
	}

	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return 0, err
	}

	if err := validateResponse(string(buf[:n]), nonce); err != nil {
		return 0, err
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

func dialTCP(ctx context.Context, addr, socksProxy string, timeout time.Duration) (net.Conn, error) {
	if socksProxy == "" {
		dialer := &net.Dialer{Timeout: timeout}
		return dialer.DialContext(ctx, "tcp", addr)
	}
	return dialViaSocks(ctx, addr, socksProxy, timeout)
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
