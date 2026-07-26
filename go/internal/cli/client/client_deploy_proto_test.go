package clientcmd

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestBoundedBufferRespectsLimit(t *testing.T) {
	var buf boundedBuffer
	buf.limit = 8

	buf.appendLine("abcd")
	buf.appendLine("efgh")
	buf.appendLine("ijkl")

	if len(buf.String()) > buf.limit {
		t.Fatalf("buffer exceeded limit: len=%d limit=%d data=%q", len(buf.String()), buf.limit, buf.String())
	}
	if !strings.HasSuffix(buf.String(), "ijkl\n") {
		t.Fatalf("buffer did not keep latest entry: %q", buf.String())
	}
}

func TestDeploySessionCompleteClosesConnection(t *testing.T) {
	client, server := net.Pipe()
	session := &tcpDeploySession{
		conn: client,
		rw:   bufio.NewReadWriter(bufio.NewReader(client), bufio.NewWriter(client)),
	}
	serverDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(server)
		line, err := reader.ReadString('\n')
		if err == nil && line != "COMPLETE OK\n" {
			err = errors.New("unexpected completion line: " + line)
		}
		if err == nil {
			_, err = server.Write([]byte("BYE\n"))
		}
		if err == nil {
			_ = server.SetReadDeadline(time.Now().Add(time.Second))
			_, err = reader.ReadByte()
			if !errors.Is(err, io.EOF) {
				err = errors.New("deploy connection remained open")
			} else {
				err = nil
			}
		}
		_ = server.Close()
		serverDone <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := session.Complete(ctx, "OK"); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

func TestBoundedBufferZeroLimit(t *testing.T) {
	var buf boundedBuffer
	buf.limit = 0

	buf.appendLine("foo")
	if buf.String() != "" {
		t.Fatalf("buffer with zero limit should stay empty, got %q", buf.String())
	}
}
