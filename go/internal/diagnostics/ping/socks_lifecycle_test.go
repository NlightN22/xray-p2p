package ping

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
)

func TestRunContinuousReusesAndClosesSOCKSConnection(t *testing.T) {
	var requests atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var request controlplane.PingRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(controlplane.PingResponse{Nonce: request.Nonce})
	}))
	defer target.Close()

	proxy := startSOCKSProxy(t)
	host, portText, err := net.SplitHostPort(target.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	err = Run(ctx, host, Options{
		Timeout:       time.Second,
		Port:          port,
		SocksProxy:    proxy.address,
		AllowInsecure: true,
		Continuous:    true,
		Silent:        true,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline, got %v", err)
	}
	if got := requests.Load(); got < 2 {
		t.Fatalf("continuous SOCKS ping sent %d requests, want at least 2", got)
	}
	if got := proxy.accepted.Load(); got != 1 {
		t.Fatalf("continuous SOCKS ping opened %d proxy connections, want 1", got)
	}
	waitForCount(t, &proxy.closed, proxy.accepted.Load())
}

type testSOCKSProxy struct {
	address  string
	accepted atomic.Int32
	closed   atomic.Int32
}

func startSOCKSProxy(t *testing.T) *testSOCKSProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxy := &testSOCKSProxy{address: listener.Addr().String()}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			proxy.accepted.Add(1)
			go proxy.serve(conn)
		}
	}()
	return proxy
}

func (p *testSOCKSProxy) serve(client net.Conn) {
	defer p.closed.Add(1)
	defer client.Close()
	target, err := readSOCKSTarget(client)
	if err != nil {
		return
	}
	upstream, err := net.DialTimeout("tcp", target, time.Second)
	if err != nil {
		return
	}
	defer upstream.Close()
	if _, err := client.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, client)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, upstream)
		done <- struct{}{}
	}()
	<-done
}

func readSOCKSTarget(conn net.Conn) (string, error) {
	greeting := make([]byte, 2)
	if _, err := io.ReadFull(conn, greeting); err != nil {
		return "", err
	}
	methods := make([]byte, int(greeting[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", err
	}
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		return "", err
	}
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", err
	}
	var host string
	switch header[3] {
	case 1:
		raw := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(conn, raw); err != nil {
			return "", err
		}
		host = net.IP(raw).String()
	case 3:
		size := make([]byte, 1)
		if _, err := io.ReadFull(conn, size); err != nil {
			return "", err
		}
		raw := make([]byte, int(size[0]))
		if _, err := io.ReadFull(conn, raw); err != nil {
			return "", err
		}
		host = string(raw)
	default:
		return "", fmt.Errorf("unsupported SOCKS address type %d", header[3])
	}
	rawPort := make([]byte, 2)
	if _, err := io.ReadFull(conn, rawPort); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(rawPort)))), nil
}

func waitForCount(t *testing.T, count *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for count.Load() != want && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := count.Load(); got != want {
		t.Fatalf("closed proxy connections = %d, want %d after Run returned", got, want)
	}
}
