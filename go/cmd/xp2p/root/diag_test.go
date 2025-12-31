package root

import (
	"bufio"
	"context"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/testutil"
)

func TestSplitListenAddress(t *testing.T) {
	tests := []struct {
		value string
		host  string
		port  string
	}{
		{value: "62022", host: "0.0.0.0", port: "62022"},
		{value: "127.0.0.1:62022", host: "127.0.0.1", port: "62022"},
	}

	for _, tt := range tests {
		host, port, err := splitListenAddress(tt.value)
		if err != nil {
			t.Fatalf("splitListenAddress(%q) returned error: %v", tt.value, err)
		}
		if host != tt.host || port != tt.port {
			t.Fatalf("splitListenAddress(%q) = %s:%s, want %s:%s", tt.value, host, port, tt.host, tt.port)
		}
	}
}

func TestRunDiagCommandTCP(t *testing.T) {
	logging.Configure(logging.Options{Output: io.Discard})
	t.Cleanup(func() {
		logging.Configure(logging.Options{Output: os.Stderr})
	})

	portStr, _ := testutil.FreePort(t)
	addr := net.JoinHostPort("127.0.0.1", portStr)
	cfg := config.Config{
		Server: config.ServerConfig{Port: portStr},
	}
	opts := diagCommandOptions{
		Listen: addr,
		Proto:  "tcp",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan int, 1)
	go func() {
		resultCh <- runDiagCommand(ctx, cfg, opts)
	}()

	testutil.WaitForCondition(t, time.Second, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	})

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("failed to dial tcp listener: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("PING\n")); err != nil {
		t.Fatalf("failed to write ping: %v", err)
	}
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read pong: %v", err)
	}
	if strings.TrimSpace(resp) != "PONG" {
		t.Fatalf("unexpected tcp response: %q", resp)
	}

	cancel()
	select {
	case code := <-resultCh:
		if code != 0 {
			t.Fatalf("runDiagCommand returned %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("runDiagCommand did not exit after context cancel")
	}
}

func TestRunDiagCommandUDP(t *testing.T) {
	logging.Configure(logging.Options{Output: io.Discard})
	t.Cleanup(func() {
		logging.Configure(logging.Options{Output: os.Stderr})
	})

	portStr, _ := testutil.FreePort(t)
	addr := net.JoinHostPort("127.0.0.1", portStr)
	cfg := config.Config{
		Server: config.ServerConfig{Port: portStr},
	}
	opts := diagCommandOptions{
		Listen: addr,
		Proto:  "udp",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan int, 1)
	go func() {
		resultCh <- runDiagCommand(ctx, cfg, opts)
	}()

	testutil.WaitForCondition(t, time.Second, func() bool {
		conn, err := net.DialTimeout("udp", addr, 50*time.Millisecond)
		if err != nil {
			return false
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(100 * time.Millisecond))
		if _, err := conn.Write([]byte("PING\n")); err != nil {
			return false
		}
		buf := make([]byte, 8)
		n, err := conn.Read(buf)
		if err != nil {
			return false
		}
		return strings.TrimSpace(string(buf[:n])) == "PONG"
	})

	cancel()
	select {
	case code := <-resultCh:
		if code != 0 {
			t.Fatalf("runDiagCommand returned %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("runDiagCommand did not exit after context cancel")
	}
}
