package servercmd

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
)

func TestDeployServerHandleConnUnauthorized(t *testing.T) {
	ctx := context.Background()
	manifest := spec.Manifest{
		Host:           "10.0.0.1",
		Version:        2,
		InstallDir:     "/srv/xp2p",
		TrojanPort:     "62022",
		TrojanUser:     "user@example.com",
		TrojanPassword: "secret",
	}
	_, enc, err := deploylink.Build(manifest.Host, "62025", manifest, time.Minute)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	srv := deployServer{
		ListenAddr: ":62025",
		Expected:   enc,
	}

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		srv.handleConn(ctx, serverConn, nil)
		close(done)
	}()

	reader := bufio.NewReader(clientConn)
	fmt.Fprint(clientConn, "AUTH\n")
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read AUTH reply: %v", err)
	}
	if strings.TrimSpace(line) != "OK" {
		t.Fatalf("unexpected AUTH reply: %q", line)
	}

	bad := []byte("bad-ciphertext")
	fmt.Fprintf(clientConn, "MANIFEST-ENC %d\n", len(bad))
	if _, err := clientConn.Write(bad); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	resp, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read error line: %v", err)
	}
	if got := strings.TrimSpace(resp); got != "ERR unauthorized" {
		t.Fatalf("unexpected response: %q", got)
	}

	clientConn.Close()
	<-done
}

func TestDeployServerHandleConnExpired(t *testing.T) {
	ctx := context.Background()
	manifest := spec.Manifest{
		Host:           "10.0.0.1",
		Version:        2,
		InstallDir:     "/srv/xp2p",
		TrojanPort:     "62022",
		TrojanUser:     "user@example.com",
		TrojanPassword: "secret",
		ExpiresAt:      time.Now().Add(-time.Minute).Unix(),
	}
	_, enc, err := deploylink.Build(manifest.Host, "62025", manifest, time.Minute)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	srv := deployServer{
		ListenAddr: ":62025",
		Expected:   enc,
	}

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		srv.handleConn(ctx, serverConn, nil)
		close(done)
	}()

	reader := bufio.NewReader(clientConn)
	fmt.Fprint(clientConn, "AUTH\n")
	if line, err := reader.ReadString('\n'); err != nil || strings.TrimSpace(line) != "OK" {
		t.Fatalf("AUTH roundtrip failed: %q err=%v", line, err)
	}

	fmt.Fprintf(clientConn, "MANIFEST-ENC %d\n", len(enc.Ciphertext))
	if _, err := clientConn.Write(enc.Ciphertext); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	resp, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read error line: %v", err)
	}
	if got := strings.TrimSpace(resp); got != "ERR link expired" {
		t.Fatalf("unexpected response: %q", got)
	}

	clientConn.Close()
	<-done
}
