package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/testutil"
)

func TestStartBackgroundServesAndShutsDown(t *testing.T) {
	logging.Configure(logging.Options{Output: io.Discard})
	t.Cleanup(func() {
		logging.Configure(logging.Options{Output: os.Stderr})
	})

	portStr, _ := testutil.FreePort(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := StartBackground(ctx, Options{Port: portStr}); err != nil {
		t.Fatalf("StartBackground returned error: %v", err)
	}

	addr := net.JoinHostPort("127.0.0.1", portStr)
	testutil.WaitForCondition(t, time.Second, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	})

	tcpConn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("failed to dial tcp server: %v", err)
	}
	t.Cleanup(func() { _ = tcpConn.Close() })

	if _, err := tcpConn.Write([]byte("PING\n")); err != nil {
		t.Fatalf("failed to write tcp request: %v", err)
	}
	resp, err := bufio.NewReader(tcpConn).ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read tcp response: %v", err)
	}
	if got := strings.TrimSpace(resp); got != pingResponse {
		t.Fatalf("unexpected tcp response: %q", got)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("failed to resolve udp address: %v", err)
	}
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.Fatalf("failed to dial udp server: %v", err)
	}
	t.Cleanup(func() { _ = udpConn.Close() })

	if err := udpConn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("failed to set udp deadline: %v", err)
	}
	if _, err := udpConn.Write([]byte("PING\n")); err != nil {
		t.Fatalf("failed to write udp request: %v", err)
	}

	udpBuf := make([]byte, 32)
	n, err := udpConn.Read(udpBuf)
	if err != nil {
		t.Fatalf("failed to read udp response: %v", err)
	}
	if got := strings.TrimSpace(string(udpBuf[:n])); got != pingResponse {
		t.Fatalf("unexpected udp response: %q", got)
	}

	cancel()
	testutil.WaitForCondition(t, time.Second, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err != nil {
			return true
		}
		_ = conn.Close()
		return false
	})

	if err := udpConn.SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatalf("failed to update udp deadline after shutdown: %v", err)
	}
	if _, err := udpConn.Write([]byte("PING\n")); err == nil {
		buf := make([]byte, 8)
		if _, err := udpConn.Read(buf); err == nil {
			t.Fatalf("expected udp read to fail after shutdown")
		}
	}
}

func TestHeartbeatPayloadIsPersisted(t *testing.T) {
	logging.Configure(logging.Options{Output: io.Discard})
	t.Cleanup(func() {
		logging.Configure(logging.Options{Output: os.Stderr})
	})

	dir := t.TempDir()
	portStr, _ := testutil.FreePort(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := StartBackground(ctx, Options{Port: portStr, InstallDir: dir}); err != nil {
		t.Fatalf("StartBackground: %v", err)
	}

	addr := net.JoinHostPort("127.0.0.1", portStr)
	testutil.WaitForCondition(t, time.Second, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	})

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("PING\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}
	reader := bufio.NewReader(conn)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("read pong: %v", err)
	}

	payload := heartbeat.Payload{
		Tag:       "proxy-test",
		Host:      "edge.example.com",
		ClientIP:  "10.0.0.5",
		Timestamp: time.Now().UTC(),
		RTTMillis: 12,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, err := conn.Write(append(data, '\n')); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	statePath := filepath.Join(dir, layout.HeartbeatStateFileName)
	testutil.WaitForCondition(t, time.Second, func() bool {
		_, err := os.Stat(statePath)
		return err == nil
	})

	state, err := heartbeat.Load(statePath)
	if err != nil {
		t.Fatalf("Load heartbeat state: %v", err)
	}
	snapshots := state.Snapshot(time.Now(), time.Second)
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	if got := snapshots[0].Entry.Host; got != payload.Host {
		t.Fatalf("unexpected host %q", got)
	}
}
