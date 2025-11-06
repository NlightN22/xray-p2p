package servercmd

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
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
	Host      string
	Key       string
	Nonce     string
	CipherB64 string
	Exp       int64
	ctHashHex string
	Manifest  deployManifest
}

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

	results := make(chan runSignal, 4)
	defer close(results)

	idleTimer := time.NewTimer(s.Timeout)
	defer idleTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig := <-results:
			if sig.ok {
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
	if err := writeLine(rw, "OK"); err != nil {
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

	header, err := readLine(rw)
	if err != nil {
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	if !strings.HasPrefix(header, "MANIFEST-ENC ") {
		_ = writeLine(rw, "ERR expected MANIFEST-ENC")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

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

	if s.Expected.Key == "" {
		_ = writeLine(rw, "ERR deploy link not configured")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}
	if s.Expected.Exp > 0 && time.Now().Unix() > s.Expected.Exp {
		_ = writeLine(rw, "ERR link expired")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

	sum := sha256.Sum256(buf)
	if hex.EncodeToString(sum[:]) != s.Expected.ctHashHex {
		_ = writeLine(rw, "ERR unauthorized")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

	recvCT := base64.RawURLEncoding.EncodeToString(buf)
	deployPort := extractPort(s.ListenAddr)
	netLink := fmt.Sprintf("xp2p+deploy://%s:%s?v=2&ct=%s&n=%s&exp=%d", s.Expected.Host, deployPort, recvCT, s.Expected.Nonce, s.Expected.Exp)
	logging.Info("xp2p server deploy: received deploy link", "link", netLink)

	manifest := s.Expected.Manifest
	if strings.TrimSpace(manifest.Host) == "" {
		manifest.Host = s.Expected.Host
	}

	s.proceedInstall(ctx, rw, results, manifest)
}

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

	linkHost := strings.TrimSpace(u.Hostname())
	q := u.Query()
	if ver := strings.TrimSpace(q.Get("v")); ver != "2" {
		return expectedLink{}, fmt.Errorf("unsupported deploy link version %q (require v=2)", ver)
	}

	key := strings.TrimSpace(q.Get("k"))
	cipherText := strings.TrimSpace(q.Get("ct"))
	nonce := strings.TrimSpace(q.Get("n"))
	if key == "" || cipherText == "" || nonce == "" {
		return expectedLink{}, fmt.Errorf("deploy link missing required parameters (k, ct, n)")
	}

	var expUnix int64
	if rawExp := strings.TrimSpace(q.Get("exp")); rawExp != "" {
		value, err := strconv.ParseInt(rawExp, 10, 64)
		if err != nil {
			return expectedLink{}, fmt.Errorf("invalid exp value %q", rawExp)
		}
		expUnix = value
	}

	plain, err := decryptManifestAESGCM(key, nonce, cipherText)
	if err != nil {
		return expectedLink{}, fmt.Errorf("invalid encrypted manifest: %w", err)
	}
	var manifest deployManifest
	if err := json.Unmarshal(plain, &manifest); err != nil {
		return expectedLink{}, fmt.Errorf("invalid manifest payload: %w", err)
	}

	manifestHost := strings.TrimSpace(manifest.Host)
	if manifestHost == "" && linkHost == "" {
		return expectedLink{}, fmt.Errorf("deploy link missing host")
	}
	if linkHost != "" && manifestHost != "" && !strings.EqualFold(linkHost, manifestHost) {
		return expectedLink{}, fmt.Errorf("link host %q mismatches manifest host %q", linkHost, manifestHost)
	}
	finalHost := manifestHost
	if finalHost == "" {
		finalHost = linkHost
	}
	if err := netutil.ValidateHost(finalHost); err != nil {
		return expectedLink{}, fmt.Errorf("invalid host %q: %w", finalHost, err)
	}

	ctBytes, err := base64.RawURLEncoding.DecodeString(cipherText)
	if err != nil {
		return expectedLink{}, fmt.Errorf("invalid ciphertext encoding: %w", err)
	}
	sum := sha256.Sum256(ctBytes)

	return expectedLink{
		Host:      finalHost,
		Key:       key,
		Nonce:     nonce,
		CipherB64: cipherText,
		Exp:       expUnix,
		ctHashHex: hex.EncodeToString(sum[:]),
		Manifest:  manifest,
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

func (s *deployServer) proceedInstall(ctx context.Context, rw *bufio.ReadWriter, results chan<- runSignal, man deployManifest) {
	host := strings.TrimSpace(man.Host)
	if host == "" {
		host = strings.TrimSpace(s.Expected.Host)
	}
	if err := netutil.ValidateHost(host); err != nil {
		_ = writeLine(rw, "ERR invalid host")
		if results != nil {
			results <- runSignal{ok: false}
		}
		return
	}

	installDir := strings.TrimSpace(man.InstallDir)
	if installDir == "" {
		installDir = strings.TrimSpace(s.Cfg.Server.InstallDir)
	}
	configDir := strings.TrimSpace(s.Cfg.Server.ConfigDir)
	if configDir == "" {
		configDir = server.DefaultServerConfigDir
	}

	port := strings.TrimSpace(man.TrojanPort)
	if port == "" {
		port = strconv.Itoa(server.DefaultTrojanPort)
	}

	logs := []string{
		fmt.Sprintf("install_dir=%s", installDir),
		fmt.Sprintf("config_dir=%s", configDir),
		fmt.Sprintf("trojan_port=%s", port),
		fmt.Sprintf("host=%s", host),
	}

	inst := server.InstallOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		Port:       port,
		Host:       host,
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

	userID := strings.TrimSpace(man.User)
	if userID == "" {
		userID = fmt.Sprintf("xp2p-%d@local", time.Now().Unix())
	}
	password := strings.TrimSpace(man.Password)
	if password == "" {
		secret, err := generateSecret(18)
		if err != nil {
			_ = writeLine(rw, "ERR generate password failed")
			if results != nil {
				results <- runSignal{ok: false}
			}
			return
		}
		password = secret
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

	link, err := server.GetUserLink(ctx, server.UserLinkOptions{InstallDir: installDir, ConfigDir: configDir, Host: host, UserID: userID})
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
	if i := strings.LastIndex(listen, ":"); i >= 0 && i+1 < len(listen) {
		return strings.TrimSpace(listen[i+1:])
	}
	return "62025"
}
