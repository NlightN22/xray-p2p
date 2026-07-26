package ping

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"

	"golang.org/x/net/proxy"
)

func pingHTTPS(ctx context.Context, addr string, timeout time.Duration, seq int, opts Options) (time.Duration, error) {
	nonce, err := newNonce()
	if err != nil {
		return 0, err
	}
	reqPayload := controlplane.PingRequest{Nonce: nonce}
	body, err := json.Marshal(reqPayload)
	if err != nil {
		return 0, err
	}
	url := "https://" + addr + controlplane.PathPing
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if opts.User != "" || opts.Credential != "" {
		if err := controlplane.ApplyHeaders(req, opts.User, opts.Credential, nonce, body, time.Now().UTC()); err != nil {
			return 0, err
		}
	}
	if opts.HTTPClient == nil {
		return 0, errors.New("HTTP client is required")
	}
	start := time.Now()
	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer ownedhttp.DrainAndClose(resp, 1<<20)
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("control ping failed: %s", resp.Status)
	}
	var pong controlplane.PingResponse
	if err := json.Unmarshal(respBody, &pong); err != nil {
		return 0, fmt.Errorf("decode control ping response: %w", err)
	}
	if pong.Nonce != nonce {
		return 0, fmt.Errorf("unexpected response nonce: %q", pong.Nonce)
	}
	rtt := time.Since(start)
	if opts.Reporter != nil {
		result := Result{Seq: seq, Target: addr, Proto: protocolHTTPS, RTT: rtt}
		if err := opts.Reporter.Report(ctx, result); err != nil {
			return rtt, err
		}
	}
	return rtt, nil
}

type pingDialer struct {
	proxyAddr string
	timeout   time.Duration
}

func newPingDialer(proxyAddr string, timeout time.Duration) *pingDialer {
	return &pingDialer{proxyAddr: proxyAddr, timeout: timeout}
}

func (d *pingDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if d.proxyAddr == "" {
		return (&net.Dialer{Timeout: d.timeout}).DialContext(ctx, network, addr)
	}
	return dialViaSocks(ctx, addr, d.proxyAddr, d.timeout)
}

func (*pingDialer) Shutdown(context.Context) error {
	return nil
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
