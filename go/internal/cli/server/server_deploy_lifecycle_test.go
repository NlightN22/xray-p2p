package servercmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestDeployServerShutdownClosesAndJoinsStalledHandler(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := probe.Addr().String()
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	server := &deployServer{ListenAddr: address, Timeout: time.Hour}
	go func() { done <- server.Run(ctx) }()

	var conn net.Conn
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err = net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if conn == nil {
		t.Fatalf("connect to deploy server: %v", err)
	}
	defer conn.Close()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context cancellation", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("deploy server retained a stalled handler during shutdown")
	}
}

func TestDeployServerShutdownWithNoHandlers(t *testing.T) {
	address := freeDeployAddress(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- (&deployServer{ListenAddr: address, Timeout: time.Hour}).Run(ctx) }()
	cancel()
	assertDeployRunCanceled(t, done)
}

func TestDeployServerShutdownClosesMultipleHandlers(t *testing.T) {
	address := freeDeployAddress(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- (&deployServer{ListenAddr: address, Timeout: time.Hour}).Run(ctx) }()
	connections := make([]net.Conn, 0, 3)
	for range 3 {
		connections = append(connections, connectDeploy(t, address))
	}
	defer func() {
		for _, conn := range connections {
			_ = conn.Close()
		}
	}()
	cancel()
	assertDeployRunCanceled(t, done)
}

func TestDeployServerShutdownCancelsAndJoinsActiveInstallHandler(t *testing.T) {
	address := freeDeployAddress(t)
	started := make(chan struct{})
	exited := make(chan struct{})
	originalInstall := serverInstallFunc
	originalInstallPresent := deployInstallPresentFunc
	t.Cleanup(func() {
		serverInstallFunc = originalInstall
		deployInstallPresentFunc = originalInstallPresent
	})
	deployInstallPresentFunc = func(clishared.InstallRole, string, string) (bool, error) {
		return false, nil
	}
	serverInstallFunc = func(ctx context.Context, _ server.InstallOptions) error {
		close(started)
		<-ctx.Done()
		close(exited)
		return ctx.Err()
	}

	manifest := spec.Manifest{
		Host:           "10.0.0.1",
		Version:        2,
		InstallDir:     t.TempDir(),
		TrojanPort:     "58443",
		TrojanUser:     "user@example.com",
		TrojanPassword: "secret",
	}
	_, encrypted, err := deploylink.Build(manifest.Host, "62025", manifest, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	server := &deployServer{
		ListenAddr: address,
		Timeout:    time.Hour,
		Expected:   encrypted,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx) }()
	conn := connectDeploy(t, address)
	defer conn.Close()
	reader := bufio.NewReader(conn)
	fmt.Fprint(conn, "AUTH\n")
	if line, readErr := reader.ReadString('\n'); readErr != nil || strings.TrimSpace(line) != "OK" {
		t.Fatalf("AUTH roundtrip failed: %q err=%v", line, readErr)
	}
	fmt.Fprintf(conn, "MANIFEST-ENC %d\n", len(encrypted.Ciphertext))
	if _, err := conn.Write(encrypted.Ciphertext); err != nil {
		t.Fatal(err)
	}
	<-started
	cancel()
	assertDeployRunCanceled(t, done)
	select {
	case <-exited:
	default:
		t.Fatal("deploy Run returned before active install handler exited")
	}
}

func freeDeployAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func connectDeploy(t *testing.T, address string) net.Conn {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			return conn
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("connect to deploy server %s timed out", address)
	return nil
}

func assertDeployRunCanceled(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context cancellation", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("deploy server shutdown did not complete")
	}
}
