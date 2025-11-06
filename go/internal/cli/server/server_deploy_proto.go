package servercmd

import (
    "bufio"
    "context"
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net"
    "net/url"
    "strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

type deployServer struct {
	ListenAddr string
	Expected   expectedLink
	Once       bool
	Timeout    time.Duration
	Cfg        config.Config
}

type expectedLink struct {
	Host       string
	Token      string
	TrojanPort string
	InstallDir string
	User       string
	Password   string
}

func (s *deployServer) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.ListenAddr)
	if err != nil {
		return err
	}
	defer ln.Close()

	type result struct{ err error }
	done := make(chan result, 1)

	go func() {
		defer close(done)
		var served int
		idleTimer := time.NewTimer(s.Timeout)
		defer idleTimer.Stop()

		for {
			ln.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
			conn, err := ln.Accept()
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					if s.Timeout > 0 {
						select {
						case <-idleTimer.C:
							logging.Info("xp2p server deploy: idle timeout reached; shutting down")
							return
						default:
						}
					}
					select {
					case <-ctx.Done():
						return
					default:
					}
					continue
				}
				done <- result{err: err}
				return
			}
			go s.handleConn(ctx, conn)
			served++
			if s.Once && served >= 1 {
				return
			}
		}
	}()

	select {
	case r := <-done:
		return r.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *deployServer) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	// AUTH
	line, err := readLine(rw)
	if err != nil {
		return
	}
	if !strings.HasPrefix(line, "AUTH") {
		_ = writeLine(rw, "ERR expected AUTH")
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(line, "AUTH"))
	if strings.HasPrefix(token, " ") {
		token = strings.TrimSpace(token)
	}
	if s.Expected.Token != "" && token != s.Expected.Token {
		_ = writeLine(rw, "ERR unauthorized")
		return
	}
	if err := writeLine(rw, "OK"); err != nil {
		return
	}

	// MANIFEST header
	header, err := readLine(rw)
	if err != nil {
		return
	}
	if !strings.HasPrefix(header, "MANIFEST ") {
		_ = writeLine(rw, "ERR expected MANIFEST")
		return
	}
	nStr := strings.TrimSpace(strings.TrimPrefix(header, "MANIFEST "))
	n, err := strconv.Atoi(nStr)
	if err != nil || n < 0 || n > 1<<20 {
		_ = writeLine(rw, "ERR invalid MANIFEST length")
		return
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(rw, buf); err != nil {
		_ = writeLine(rw, "ERR read MANIFEST body failed")
		return
	}
	var man struct {
		Host       string `json:"host"`
		Version    int    `json:"version"`
		TrojanPort string `json:"trojan_port"`
		InstallDir string `json:"install_dir"`
		User       string `json:"user"`
		Password   string `json:"password"`
	}
    if err := json.Unmarshal(buf, &man); err != nil {
        _ = writeLine(rw, "ERR parse MANIFEST failed")
        return
    }
	if err := netutil.ValidateHost(man.Host); err != nil {
		_ = writeLine(rw, "ERR invalid host")
		return
	}
    if s.Expected.Host != "" && !strings.EqualFold(strings.TrimSpace(s.Expected.Host), strings.TrimSpace(man.Host)) {
        _ = writeLine(rw, "ERR host mismatch")
        return
    }
    if s.Expected.User != "" && strings.TrimSpace(man.User) != "" && !strings.EqualFold(strings.TrimSpace(man.User), strings.TrimSpace(s.Expected.User)) {
        _ = writeLine(rw, "ERR user mismatch")
        return
    }
    if s.Expected.Password != "" && strings.TrimSpace(man.Password) != "" && strings.TrimSpace(man.Password) != strings.TrimSpace(s.Expected.Password) {
        _ = writeLine(rw, "ERR password mismatch")
        return
    }

	// Installation
	_ = writeLine(rw, "RUN")

	installDir := firstNonEmpty(man.InstallDir, s.Expected.InstallDir, s.Cfg.Server.InstallDir)
	configDir := s.Cfg.Server.ConfigDir
	port := firstNonEmpty(man.TrojanPort, s.Expected.TrojanPort)

	if port == "" {
		port = strconv.Itoa(server.DefaultTrojanPort)
	}

	logs := []string{
		fmt.Sprintf("install_dir=%s", installDir),
		fmt.Sprintf("config_dir=%s", configDir),
		fmt.Sprintf("trojan_port=%s", port),
		fmt.Sprintf("host=%s", man.Host),
	}

	inst := server.InstallOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		Port:       port,
		Host:       man.Host,
		Force:      true,
	}

	if err := server.Install(ctx, inst); err != nil {
		_ = writeLine(rw, "EXIT 1")
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{err.Error()})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		return
	}

	// User provisioning
	userID := firstNonEmpty(strings.TrimSpace(man.User), strings.TrimSpace(s.Expected.User))
	if userID == "" {
		userID = fmt.Sprintf("xp2p-%d@local", time.Now().Unix())
	}
	password := firstNonEmpty(strings.TrimSpace(man.Password), strings.TrimSpace(s.Expected.Password))
	if password == "" {
		password, _ = generateSecret(18)
	}

	if err := server.AddUser(ctx, server.AddUserOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		UserID:     userID,
		Password:   password,
	}); err != nil {
		_ = writeLine(rw, "EXIT 1")
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{err.Error()})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		return
	}

	link, err := server.GetUserLink(ctx, server.UserLinkOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		Host:       man.Host,
		UserID:     userID,
	})
	if err != nil || strings.TrimSpace(link.Link) == "" {
		_ = writeLine(rw, "EXIT 1")
		reason := "failed to build user link"
		if err != nil {
			reason = err.Error()
		}
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{reason})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		return
	}

	_ = writeLine(rw, "EXIT 0")
	_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
	_ = writeLine(rw, "LINK "+link.Link)
	_ = writeLine(rw, "DONE")
}

// --- link parsing and helpers ---

func parseDeployLink(raw string) (expectedLink, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return expectedLink{}, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return expectedLink{}, err
	}
	if !strings.EqualFold(u.Scheme, "xp2p+deploy") {
		return expectedLink{}, fmt.Errorf("invalid scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host != "" && netutil.ValidateHost(host) != nil {
		return expectedLink{}, fmt.Errorf("invalid host in link")
	}
	q := u.Query()
	return expectedLink{
		Host:       host,
		Token:      strings.TrimSpace(q.Get("token")),
		TrojanPort: strings.TrimSpace(q.Get("tp")),
		InstallDir: strings.TrimSpace(q.Get("idir")),
		User:       strings.TrimSpace(q.Get("user")),
		Password:   strings.TrimSpace(q.Get("pass")),
	}, nil
}

func readLine(rw *bufio.ReadWriter) (string, error) {
	s, err := rw.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}

func writeLine(rw *bufio.ReadWriter, line string) error {
	if _, err := rw.WriteString(line + "\n"); err != nil {
		return err
	}
	return rw.Flush()
}

func writeSegment(rw *bufio.ReadWriter, begin, end string, lines []string) error {
	if err := writeLine(rw, begin); err != nil {
		return err
	}
	for _, l := range lines {
		if _, err := rw.WriteString(l + "\n"); err != nil {
			return err
		}
	}
	if err := rw.Flush(); err != nil {
		return err
	}
	return writeLine(rw, end)
}

func generateSecret(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
