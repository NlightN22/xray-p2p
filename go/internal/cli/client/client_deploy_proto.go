package clientcmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const (
	deployDialTimeout = 10 * time.Second
	deployIOTimeout   = 60 * time.Second
	deployBufferLimit = 64 * 1024 // 64KB cap for OUT/ERR buffers
)

type deployManifest struct {
	Host       string `json:"host"`
	Version    int    `json:"version"`
	TrojanPort string `json:"trojan_port,omitempty"`
	InstallDir string `json:"install_dir,omitempty"`
	User       string `json:"user,omitempty"`
	Password   string `json:"password,omitempty"`
}

type deployResult struct {
	ExitCode int
	Link     string
	OutLog   string
	ErrLog   string
}

func performDeployHandshake(ctx context.Context, opts deployOptions) (deployResult, error) {
	addr := net.JoinHostPort(strings.TrimSpace(opts.runtime.remoteHost), strings.TrimSpace(opts.runtime.deployPort))

	d := &net.Dialer{Timeout: deployDialTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return deployResult{}, fmt.Errorf("connect to %s: %w", addr, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(deployIOTimeout)); err != nil {
		return deployResult{}, err
	}

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

    // AUTH with token when present
    if _, err := fmt.Fprintf(rw, "AUTH %s\n", strings.TrimSpace(opts.runtime.token)); err != nil {
        return deployResult{}, fmt.Errorf("send AUTH: %w", err)
    }
	if err := rw.Flush(); err != nil {
		return deployResult{}, fmt.Errorf("flush AUTH: %w", err)
	}

	line, err := readLine(rw)
	if err != nil {
		return deployResult{}, fmt.Errorf("read AUTH response: %w", err)
	}
	if !strings.HasPrefix(line, "OK") {
		if strings.HasPrefix(line, "ERR ") {
			return deployResult{}, fmt.Errorf("server error: %s", strings.TrimSpace(strings.TrimPrefix(line, "ERR ")))
		}
		return deployResult{}, fmt.Errorf("unexpected AUTH response: %q", line)
	}

	// MANIFEST
	man := deployManifest{
		Host:       strings.TrimSpace(opts.runtime.serverHost),
		Version:    1,
		TrojanPort: strings.TrimSpace(opts.manifest.trojanPort),
		InstallDir: strings.TrimSpace(opts.manifest.installDir),
		User:       strings.TrimSpace(opts.manifest.trojanUser),
		Password:   strings.TrimSpace(opts.manifest.trojanPassword),
	}
	payload, err := json.Marshal(man)
	if err != nil {
		return deployResult{}, fmt.Errorf("encode manifest: %w", err)
	}
	if _, err := fmt.Fprintf(rw, "MANIFEST %d\n", len(payload)); err != nil {
		return deployResult{}, fmt.Errorf("send MANIFEST header: %w", err)
	}
	if _, err := rw.Write(payload); err != nil {
		return deployResult{}, fmt.Errorf("send MANIFEST body: %w", err)
	}
	if err := rw.Flush(); err != nil {
		return deployResult{}, fmt.Errorf("flush MANIFEST: %w", err)
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
			return deployResult{}, err
		}
		l, err := readLine(rw)
		if err != nil {
			if errors.Is(err, errEOF) {
				break
			}
			return deployResult{}, err
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
				return deployResult{}, err
			}
		case l == "ERR-BEGIN":
			if err := readSegment(rw, "ERR-END", func(line string) {
				logging.Warn("server", "err", line)
				errBuf.appendLine(line)
			}); err != nil {
				return deployResult{}, err
			}
		case strings.HasPrefix(l, "LINK "):
			link = strings.TrimSpace(strings.TrimPrefix(l, "LINK "))
			logging.Info("xp2p client deploy: trojan link received", "link", link)
		case l == "DONE":
			return deployResult{ExitCode: exitCode, Link: link, OutLog: outBuf.String(), ErrLog: errBuf.String()}, nil
		case strings.HasPrefix(l, "ERR "):
			return deployResult{}, fmt.Errorf("server error: %s", strings.TrimSpace(strings.TrimPrefix(l, "ERR ")))
		default:
			// Unknown line, keep a trace to help debugging but avoid spam.
			logging.Debug("xp2p client deploy: unhandled line", "line", l)
		}
	}

	return deployResult{ExitCode: exitCode, Link: link, OutLog: outBuf.String(), ErrLog: errBuf.String()}, nil
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
