package clientcmd

import (
	"strings"
	"testing"
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

func TestBoundedBufferZeroLimit(t *testing.T) {
	var buf boundedBuffer
	buf.limit = 0

	buf.appendLine("foo")
	if buf.String() != "" {
		t.Fatalf("buffer with zero limit should stay empty, got %q", buf.String())
	}
}
