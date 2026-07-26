package clientcmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const (
	deployDialTimeout = 10 * time.Second
	deployIOTimeout   = 60 * time.Second
	deployBufferLimit = 64 * 1024 // 64KB cap for OUT/ERR buffers
)

type deploySession interface {
	Complete(context.Context, string) error
	Close() error
}

type tcpDeploySession struct {
	conn         net.Conn
	rw           *bufio.ReadWriter
	completeOnce sync.Once
	closeOnce    sync.Once
	completeErr  error
	closeErr     error
}

type deployResult struct {
	ExitCode int
	Link     string
	OutLog   string
	ErrLog   string
}

func performDeployHandshake(ctx context.Context, opts deployOptions) (deployResult, deploySession, error) {
	addr := net.JoinHostPort(strings.TrimSpace(opts.runtime.remoteHost), strings.TrimSpace(opts.runtime.deployPort))

	d := &net.Dialer{Timeout: deployDialTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return deployResult{}, nil, fmt.Errorf("connect to %s: %w", addr, err)
	}
	closeConn := func() {
		if conn != nil {
			_ = conn.Close()
			conn = nil
		}
	}

	if err := conn.SetDeadline(time.Now().Add(deployIOTimeout)); err != nil {
		closeConn()
		return deployResult{}, nil, err
	}

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	// AUTH (no token for v2)
	if _, err := fmt.Fprintf(rw, "AUTH\n"); err != nil {
		closeConn()
		return deployResult{}, nil, fmt.Errorf("send AUTH: %w", err)
	}
	if err := rw.Flush(); err != nil {
		closeConn()
		return deployResult{}, nil, fmt.Errorf("flush AUTH: %w", err)
	}

	line, err := readLine(rw)
	if err != nil {
		closeConn()
		return deployResult{}, nil, fmt.Errorf("read AUTH response: %w", err)
	}
	if !strings.HasPrefix(line, "OK") {
		if strings.HasPrefix(line, "ERR ") {
			closeConn()
			return deployResult{}, nil, serverDeployError{msg: strings.TrimSpace(strings.TrimPrefix(line, "ERR "))}
		}
		closeConn()
		return deployResult{}, nil, fmt.Errorf("unexpected AUTH response: %q", line)
	}

	if len(opts.runtime.ciphertext) == 0 {
		closeConn()
		return deployResult{}, nil, fmt.Errorf("encrypted manifest missing")
	}
	if _, err := fmt.Fprintf(rw, "MANIFEST-ENC %d\n", len(opts.runtime.ciphertext)); err != nil {
		closeConn()
		return deployResult{}, nil, fmt.Errorf("send MANIFEST-ENC header: %w", err)
	}
	if _, err := rw.Write(opts.runtime.ciphertext); err != nil {
		closeConn()
		return deployResult{}, nil, fmt.Errorf("send MANIFEST-ENC body: %w", err)
	}
	if err := rw.Flush(); err != nil {
		closeConn()
		return deployResult{}, nil, fmt.Errorf("flush MANIFEST-ENC: %w", err)
	}

	// Process server responses
	var (
		exitCode = -1
		link     string
		outBuf   boundedBuffer
		errBuf   boundedBuffer
	)

	outBuf.limit = deployBufferLimit
	errBuf.limit = deployBufferLimit

	for {
		if err := conn.SetDeadline(time.Now().Add(deployIOTimeout)); err != nil {
			closeConn()
			return deployResult{}, nil, err
		}
		l, err := readLine(rw)
		if err != nil {
			if errors.Is(err, errEOF) {
				break
			}
			closeConn()
			return deployResult{}, nil, err
		}

		switch {
		case l == "RUN":
			logging.Info("xp2p client deploy: server started install")
		case strings.HasPrefix(l, "EXIT "):
			codeStr := strings.TrimSpace(strings.TrimPrefix(l, "EXIT "))
			if n, convErr := strconv.Atoi(codeStr); convErr == nil {
				exitCode = n
			} else {
				logging.Warn("xp2p client deploy: bad EXIT code", "value", codeStr)
			}
		case l == "OUT-BEGIN":
			if err := readSegment(rw, "OUT-END", func(line string) {
				logging.Info("server", "out", line)
				outBuf.appendLine(line)
			}); err != nil {
				closeConn()
				return deployResult{}, nil, err
			}
		case l == "ERR-BEGIN":
			if err := readSegment(rw, "ERR-END", func(line string) {
				logging.Warn("server", "err", line)
				errBuf.appendLine(line)
			}); err != nil {
				closeConn()
				return deployResult{}, nil, err
			}
		case strings.HasPrefix(l, "LINK "):
			link = strings.TrimSpace(strings.TrimPrefix(l, "LINK "))
			logging.Info("xp2p client deploy: connection link received", "link", link)
		case l == "DONE":
			result := deployResult{ExitCode: exitCode, Link: link, OutLog: outBuf.String(), ErrLog: errBuf.String()}
			session := &tcpDeploySession{conn: conn, rw: rw}
			conn = nil
			return result, session, nil
		case strings.HasPrefix(l, "ERR "):
			closeConn()
			return deployResult{}, nil, serverDeployError{msg: strings.TrimSpace(strings.TrimPrefix(l, "ERR "))}
		default:
			// Unknown line, keep a trace to help debugging but avoid spam.
			logging.Debug("xp2p client deploy: unhandled line", "line", l)
		}
	}

	closeConn()
	return deployResult{ExitCode: exitCode, Link: link, OutLog: outBuf.String(), ErrLog: errBuf.String()}, nil, nil
}

func (s *tcpDeploySession) Complete(ctx context.Context, status string) error {
	s.completeOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}
		if status = strings.TrimSpace(status); status == "" {
			status = "OK"
		}
		deadline := time.Now().Add(deployCompletionNotifyTimeout)
		if value, ok := ctx.Deadline(); ok {
			deadline = value
		}
		if err := s.conn.SetDeadline(deadline); err != nil {
			s.completeErr = err
			_ = s.Close()
			return
		}
		if _, err := fmt.Fprintf(s.rw, "COMPLETE %s\n", status); err != nil {
			s.completeErr = fmt.Errorf("send COMPLETE: %w", err)
			_ = s.Close()
			return
		}
		if err := s.rw.Flush(); err != nil {
			s.completeErr = fmt.Errorf("flush COMPLETE: %w", err)
			_ = s.Close()
			return
		}
		reply, err := readLine(s.rw)
		_ = s.Close()
		if err != nil {
			s.completeErr = fmt.Errorf("read COMPLETE ack: %w", err)
			return
		}
		if strings.TrimSpace(reply) != "BYE" {
			s.completeErr = fmt.Errorf("unexpected COMPLETE ack: %q", reply)
		}
	})
	return s.completeErr
}

func (s *tcpDeploySession) Close() error {
	s.closeOnce.Do(func() {
		if s.conn != nil {
			s.closeErr = s.conn.Close()
		}
	})
	return s.closeErr
}

// --- helpers ---

var errEOF = errors.New("eof")

func readLine(rw *bufio.ReadWriter) (string, error) {
	s, err := rw.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			if s == "" {
				return "", errEOF
			}
		} else {
			return "", err
		}
	}
	return strings.TrimRight(s, "\r\n"), nil
}

func readSegment(rw *bufio.ReadWriter, end string, onLine func(string)) error {
	for {
		s, err := rw.ReadString('\n')
		if err != nil {
			return err
		}
		line := strings.TrimRight(s, "\r\n")
		if line == end {
			return nil
		}
		if onLine != nil {
			onLine(line)
		}
	}
}

type boundedBuffer struct {
	data  []byte
	limit int
}

type serverDeployError struct {
	msg string
}

func (e serverDeployError) Error() string {
	return "server error: " + e.msg
}

func isServerDeployError(err error) bool {
	var target serverDeployError
	return errors.As(err, &target)
}

func (b *boundedBuffer) appendLine(line string) {
	if b.limit <= 0 {
		return
	}
	// include newline for readability
	s := line + "\n"
	// trim from the front if needed
	if len(b.data)+len(s) > b.limit {
		// remove oldest bytes
		drop := len(b.data) + len(s) - b.limit
		if drop > len(b.data) {
			drop = len(b.data)
		}
		b.data = b.data[drop:]
	}
	b.data = append(b.data, s...)
}

func (b *boundedBuffer) String() string {
	return string(b.data)
}
