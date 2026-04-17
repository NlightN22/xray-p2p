package server

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func TestStreamPipeWritesExtraAndLogs(t *testing.T) {
	t.Run("stderr", func(t *testing.T) {
		var logBuf bytes.Buffer
		logging.Configure(logging.Options{Output: &logBuf, Format: logging.FormatText, Level: "debug"})
		defer logging.Configure(logging.Options{Output: os.Stderr, Format: logging.FormatText})

		var extra bytes.Buffer
		streamPipe(strings.NewReader(" hello \n\nworld\n"), "stderr", &extra)

		if got := extra.String(); got != "hello\nworld\n" {
			t.Fatalf("extra log = %q, want %q", got, "hello\nworld\n")
		}

		out := logBuf.String()
		if !strings.Contains(out, "ERROR xray_core stderr: hello") {
			t.Fatalf("stderr log missing hello: %s", out)
		}
		if !strings.Contains(out, "ERROR xray_core stderr: world") {
			t.Fatalf("stderr log missing world: %s", out)
		}
	})

	t.Run("stdout", func(t *testing.T) {
		var logBuf bytes.Buffer
		logging.Configure(logging.Options{Output: &logBuf, Format: logging.FormatText, Level: "debug"})
		defer logging.Configure(logging.Options{Output: os.Stderr, Format: logging.FormatText})

		streamPipe(strings.NewReader("ok\n"), "stdout", nil)

		out := logBuf.String()
		if !strings.Contains(out, "INFO xray_core stdout: ok") {
			t.Fatalf("stdout log missing ok: %s", out)
		}
	})
}
