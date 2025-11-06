package servercmd

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
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

	// Token policy (one-time with TTL)
	TokenTTL    time.Duration
	tokenIssued time.Time
	mu          sync.Mutex
	tokenUsed   bool
}

type expectedLink struct {
	Host       string
	Token      string
	TrojanPort string
	InstallDir string
	User       string
	Password   string
	Sig        string
	Key        string
	Version    int
	Nonce      string
	CipherB64  string
	Exp        int64
	ctHashHex  string
}

// runSignal is sent from a handled connection to the accept loop
// to start xray-core in the foreground with the installed paths.
type runSignal struct {
	ok         bool
	installDir string
	configDir  string
}

func (s *deployServer) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.ListenAddr)
	if err != nil {
		return err
	}
	defer ln.Close()

	// initialize token issuance time and default TTL
	if s.Expected.Token != "" && s.TokenTTL <= 0 {
		s.TokenTTL = 10 * time.Minute
	}
	s.tokenIssued = time.Now()

	results := make(chan runSignal, 4)
	defer close(results)

	idleTimer := time.NewTimer(s.Timeout)
	defer idleTimer.Stop()

	for {
		// stop on context cancel
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// stop on successful session when --once
		select {
		case sig := <-results:
			if sig.ok {
				if s.Expected.Token != "" {
					s.mu.Lock()
					s.tokenUsed = true
					s.mu.Unlock()
				}
				// Start diagnostics responders for ping on configured port.
				if err := server.StartBackground(ctx, server.Options{Port: s.Cfg.Server.Port}); err != nil {
					logging.Warn("xp2p server deploy: diagnostics start failed", "err", err)
				}
				logging.Info("xp2p server deploy: starting xray-core", "install_dir", sig.installDir, "config_dir", sig.configDir)
				if err := server.Run(ctx, server.RunOptions{InstallDir: sig.installDir, ConfigDir: sig.configDir}); err != nil {
					logging.Error("xp2p server deploy: xray-core start failed", "err", err)
				}
				if s.Once {
					return nil
				}
			}
		default:
		}

		// idle timeout
		if s.Timeout > 0 {
			select {
			case <-idleTimer.C:
				logging.Info("xp2p server deploy: idle timeout reached; shutting down")
				return nil
			default:
			}
		}

		_ = ln.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
		conn, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}
		go s.handleConn(ctx, conn, results)
	}
}

func (s *deployServer) handleConn(ctx context.Context, conn net.Conn, results chan<- runSignal) {
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	// AUTH
	line, err := readLine(rw)
	if err != nil {
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	if !strings.HasPrefix(line, "AUTH") {
		_ = writeLine(rw, "ERR expected AUTH")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(line, "AUTH"))
	if strings.HasPrefix(token, " ") {
		token = strings.TrimSpace(token)
	}
	if s.Expected.Token != "" {
		s.mu.Lock()
		used := s.tokenUsed
		issued := s.tokenIssued
		ttl := s.TokenTTL
		s.mu.Unlock()
		if used {
			_ = writeLine(rw, "ERR unauthorized")
			if results != nil {
				results <- runSignal{ok: false}
			}
			return
		}
		if token != s.Expected.Token {
			_ = writeLine(rw, "ERR unauthorized")
			if results != nil {
				results <- runSignal{ok: false}
			}
			return
		}
		if ttl > 0 && time.Since(issued) > ttl {
			_ = writeLine(rw, "ERR token expired")
			if results != nil {
				results <- runSignal{ok: false}
			}
			return
		}
	}
	if err := writeLine(rw, "OK"); err != nil {
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

	// MANIFEST header
	header, err := readLine(rw)
	if err != nil {
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	if strings.HasPrefix(header, "MANIFEST-ENC ") {
		// read ciphertext and compare against expected
		nStr := strings.TrimSpace(strings.TrimPrefix(header, "MANIFEST-ENC "))
		n, err := strconv.Atoi(nStr)
		if err != nil || n <= 0 || n > 1<<20 {
			_ = writeLine(rw, "ERR invalid MANIFEST-ENC length")
			if results != nil {
				results <- runSignal{ok: false}
			}
			return
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(rw, buf); err != nil {
			_ = writeLine(rw, "ERR read MANIFEST-ENC body failed")
			if results != nil {
				results <- runSignal{ok: false}
			}
			return
		}
		// log received network link (without key)
		recvCT := base64.RawURLEncoding.EncodeToString(buf)
		deployPort := extractPort(s.ListenAddr)
		netLink := fmt.Sprintf("xp2p+deploy://%s:%s?v=2&ct=%s&n=%s&exp=%d", s.Expected.Host, deployPort, recvCT, s.Expected.Nonce, s.Expected.Exp)
		logging.Info("xp2p server deploy: received deploy link", "link", netLink)

		if s.Expected.Version >= 2 {
			sum := sha256.Sum256(buf)
			if hex.EncodeToString(sum[:]) != strings.TrimSpace(s.Expected.ctHashHex) {
				_ = writeLine(rw, "ERR unauthorized")
				if results != nil {
					results <- runSignal{ok: false}
				}
				return
			}
			// decrypt to manifest
			plain, err := decryptManifestAESGCM(s.Expected.Key, s.Expected.Nonce, s.Expected.CipherB64)
			if err != nil {
				_ = writeLine(rw, "ERR decrypt failed")
				if results != nil {
					results <- runSignal{ok: false}
				}
				return
			}
			var man deployManifest
			if err := json.Unmarshal(plain, &man); err != nil {
				_ = writeLine(rw, "ERR decode manifest failed")
				if results != nil {
					results <- runSignal{ok: false}
				}
				return
			}
			// continue with install using decrypted manifest
			s.proceedInstall(ctx, rw, results, man)
			return
		}
		_ = writeLine(rw, "ERR unexpected MANIFEST-ENC")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	if !strings.HasPrefix(header, "MANIFEST ") {
		_ = writeLine(rw, "ERR expected MANIFEST")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	nStr := strings.TrimSpace(strings.TrimPrefix(header, "MANIFEST "))
	n, err := strconv.Atoi(nStr)
	if err != nil || n < 0 || n > 1<<20 {
		_ = writeLine(rw, "ERR invalid MANIFEST length")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(rw, buf); err != nil {
		_ = writeLine(rw, "ERR read MANIFEST body failed")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	var man deployManifest
	if err := json.Unmarshal(buf, &man); err != nil {
		_ = writeLine(rw, "ERR parse MANIFEST failed")
		return
	}
	if err := netutil.ValidateHost(man.Host); err != nil {
		_ = writeLine(rw, "ERR invalid host")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	if s.Expected.Host != "" && !strings.EqualFold(strings.TrimSpace(s.Expected.Host), strings.TrimSpace(man.Host)) {
		_ = writeLine(rw, "ERR host mismatch")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	if s.Expected.User != "" && strings.TrimSpace(man.User) != "" && !strings.EqualFold(strings.TrimSpace(man.User), strings.TrimSpace(s.Expected.User)) {
		_ = writeLine(rw, "ERR user mismatch")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	if s.Expected.Password != "" && strings.TrimSpace(man.Password) != "" && strings.TrimSpace(man.Password) != strings.TrimSpace(s.Expected.Password) {
		_ = writeLine(rw, "ERR password mismatch")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

	// Installation
	s.proceedInstall(ctx, rw, results, man)
	return
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
		if results != nil {
			results <- runSignal{ok: false}
		}
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
		if results != nil {
			results <- runSignal{ok: false}
		}
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
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

	_ = writeLine(rw, "EXIT 0")
	_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
	_ = writeLine(rw, "LINK "+link.Link)
	_ = writeLine(rw, "DONE")
	if results != nil {
		results <- runSignal{ok: true, installDir: installDir, configDir: configDir}
	}
	return
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
	ver := 1
	if v := strings.TrimSpace(q.Get("v")); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			ver = n
		}
	}
	exp := expectedLink{
		Host:       host,
		Token:      strings.TrimSpace(q.Get("token")),
		TrojanPort: strings.TrimSpace(q.Get("tp")),
		InstallDir: strings.TrimSpace(q.Get("idir")),
		User:       strings.TrimSpace(q.Get("user")),
		Password:   strings.TrimSpace(q.Get("pass")),
		Sig:        strings.TrimSpace(q.Get("sig")),
		Key:        strings.TrimSpace(q.Get("key")),
		Version:    ver,
		Nonce:      strings.TrimSpace(q.Get("n")),
		CipherB64:  strings.TrimSpace(q.Get("ct")),
	}
	if exp.Version >= 2 {
		if exp.Key == "" || exp.CipherB64 == "" || exp.Nonce == "" {
			return expectedLink{}, fmt.Errorf("v=2 requires k, ct, n")
		}
		// compute hash of ciphertext for later matching
		if ct, err := base64.RawURLEncoding.DecodeString(exp.CipherB64); err == nil {
			sum := sha256.Sum256(ct)
			exp.ctHashHex = hex.EncodeToString(sum[:])
		}
		// optional: parse exp
		if rawExp := strings.TrimSpace(q.Get("exp")); rawExp != "" {
			if n, err := strconv.ParseInt(rawExp, 10, 64); err == nil {
				exp.Exp = n
			}
		}
		// decrypt once at startup to validate manifest fields
		if _, err := decryptManifestAESGCM(exp.Key, exp.Nonce, exp.CipherB64); err != nil {
			return expectedLink{}, fmt.Errorf("invalid encrypted manifest: %w", err)
		}
	} else {
		// If sig/key present, verify that sig = HMAC_SHA256(key, token)
		if (exp.Sig != "" || exp.Key != "") && !(exp.Sig != "" && exp.Key != "") {
			return expectedLink{}, fmt.Errorf("sig/key must be both present or absent")
		}
		if exp.Sig != "" {
			calc, err := hmacSHA256Hex(exp.Key, exp.Token)
			if err != nil {
				return expectedLink{}, fmt.Errorf("invalid key in link: %w", err)
			}
			if !strings.EqualFold(calc, exp.Sig) {
				return expectedLink{}, fmt.Errorf("invalid signature in link")
			}
		}
	}
	return exp, nil
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

func hmacSHA256Hex(keyBase64URL, data string) (string, error) {
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(keyBase64URL))
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(data))
	sum := mac.Sum(nil)
	return hex.EncodeToString(sum), nil
}

// v2 decryption helpers
func decryptManifestAESGCM(keyB64, nonceB64, ctB64 string) ([]byte, error) {
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(keyB64))
	if err != nil {
		return nil, err
	}
	nonce, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(nonceB64))
	if err != nil {
		return nil, err
	}
	ct, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(ctB64))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	aad := []byte("XP2PDEPLOY|v=2")
	return aead.Open(nil, nonce, ct, aad)
}

// proceedInstall continues with installation and responds over rw; shared for v1/v2
func (s *deployServer) proceedInstall(ctx context.Context, rw *bufio.ReadWriter, results chan<- runSignal, man deployManifest) {
	if err := netutil.ValidateHost(man.Host); err != nil {
		_ = writeLine(rw, "ERR invalid host")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	if s.Expected.Host != "" && !strings.EqualFold(strings.TrimSpace(s.Expected.Host), strings.TrimSpace(man.Host)) {
		_ = writeLine(rw, "ERR host mismatch")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

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

	inst := server.InstallOptions{InstallDir: installDir, ConfigDir: configDir, Port: port, Host: man.Host, Force: true}
	if err := server.Install(ctx, inst); err != nil {
		_ = writeLine(rw, "EXIT 1")
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{err.Error()})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

	userID := firstNonEmpty(strings.TrimSpace(man.User), strings.TrimSpace(s.Expected.User))
	if userID == "" {
		userID = fmt.Sprintf("xp2p-%d@local", time.Now().Unix())
	}
	password := firstNonEmpty(strings.TrimSpace(man.Password), strings.TrimSpace(s.Expected.Password))
	if password == "" {
		if p, err := generateSecret(18); err == nil {
			password = p
		}
	}

	if err := server.AddUser(ctx, server.AddUserOptions{InstallDir: installDir, ConfigDir: configDir, UserID: userID, Password: password}); err != nil {
		_ = writeLine(rw, "EXIT 1")
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{err.Error()})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

	link, err := server.GetUserLink(ctx, server.UserLinkOptions{InstallDir: installDir, ConfigDir: configDir, Host: man.Host, UserID: userID})
	if err != nil || strings.TrimSpace(link.Link) == "" {
		_ = writeLine(rw, "EXIT 1")
		reason := "failed to build user link"
		if err != nil {
			reason = err.Error()
		}
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{reason})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

	_ = writeLine(rw, "EXIT 0")
	_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
	_ = writeLine(rw, "LINK "+link.Link)
	_ = writeLine(rw, "DONE")
	if results != nil {
		results <- runSignal{ok: true, installDir: installDir, configDir: configDir}
	}
}

func extractPort(listen string) string {
	// expect ":port" or "host:port"; fallback default deploy port
	if i := strings.LastIndex(listen, ":"); i >= 0 && i+1 < len(listen) {
		return strings.TrimSpace(listen[i+1:])
	}
	return "62025"
}
