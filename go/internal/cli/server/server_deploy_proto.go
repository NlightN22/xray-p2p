package servercmd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

const (
	serverDeployIOTimeout = 60 * time.Second
)

func (s *deployServer) handleConn(ctx context.Context, conn net.Conn, results chan<- runSignal) {
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(serverDeployIOTimeout))
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	line, err := readLine(rw)
	if err != nil {
		notifyFailure(results)
		return
	}
	if !strings.HasPrefix(line, "AUTH") {
		_ = writeLine(rw, "ERR expected AUTH")
		notifyFailure(results)
		return
	}
	if err := writeLine(rw, "OK"); err != nil {
		notifyFailure(results)
		return
	}

	header, err := readLine(rw)
	if err != nil {
		notifyFailure(results)
		return
	}
	if !strings.HasPrefix(header, "MANIFEST-ENC ") {
		_ = writeLine(rw, "ERR expected MANIFEST-ENC")
		notifyFailure(results)
		return
	}
	nStr := strings.TrimSpace(strings.TrimPrefix(header, "MANIFEST-ENC "))
	n, err := strconv.Atoi(nStr)
	if err != nil || n <= 0 || n > 1<<20 {
		_ = writeLine(rw, "ERR invalid MANIFEST-ENC length")
		notifyFailure(results)
		return
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(rw, buf); err != nil {
		_ = writeLine(rw, "ERR read MANIFEST-ENC body failed")
		notifyFailure(results)
		return
	}

	cipherB64 := base64.RawURLEncoding.EncodeToString(buf)
	logHost := strings.TrimSpace(firstNonEmpty(s.Expected.Host, s.Cfg.Server.Host))
	deployPort := extractPort(s.ListenAddr)
	netLink := deploylink.RedactedURL(logHost, deployPort, s.Expected.Nonce, s.Expected.ExpiresAt, cipherB64)
	logging.Info("xp2p server deploy: received deploy link", "link", netLink)

	if strings.TrimSpace(s.Expected.Key) == "" || len(s.Expected.Ciphertext) == 0 {
		_ = writeLine(rw, "ERR deploy link not configured")
		notifyFailure(results)
		return
	}
	if s.Expected.ExpiresAt > 0 && time.Now().Unix() > s.Expected.ExpiresAt {
		_ = writeLine(rw, "ERR link expired")
		notifyFailure(results)
		return
	}
	if !bytes.Equal(buf, s.Expected.Ciphertext) {
		_ = writeLine(rw, "ERR unauthorized")
		notifyFailure(results)
		return
	}

	manifest := s.Expected.Manifest
	if strings.TrimSpace(manifest.Host) == "" {
		manifest.Host = s.Expected.Host
	}

	s.proceedInstall(ctx, rw, results, manifest)
}

func parseDeployLink(raw string) (deploylink.EncryptedLink, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return deploylink.EncryptedLink{}, nil
	}

	enc, err := deploylink.Parse(raw)
	if err != nil {
		return deploylink.EncryptedLink{}, err
	}
	if err := netutil.ValidateHost(enc.Host); err != nil {
		return deploylink.EncryptedLink{}, err
	}
	return enc, nil
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

func notifyFailure(results chan<- runSignal) {
	if results != nil {
		results <- runSignal{ok: false}
	}
}

func generateSecret(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *deployServer) proceedInstall(ctx context.Context, rw *bufio.ReadWriter, results chan<- runSignal, man spec.Manifest) {
	host := strings.TrimSpace(man.Host)
	if host == "" {
		host = strings.TrimSpace(s.Expected.Host)
	}
	if err := netutil.ValidateHost(host); err != nil {
		_ = writeLine(rw, "ERR invalid host")
		notifyFailure(results)
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
		notifyFailure(results)
		return
	}

	userID := strings.TrimSpace(man.TrojanUser)
	if userID == "" {
		userID = fmt.Sprintf("xp2p-%d@local", time.Now().Unix())
	}
	password := strings.TrimSpace(man.TrojanPassword)
	if password == "" {
		secret, err := generateSecret(18)
		if err != nil {
			_ = writeLine(rw, "ERR generate password failed")
			notifyFailure(results)
			return
		}
		password = secret
	}

	if err := server.AddUser(ctx, server.AddUserOptions{InstallDir: installDir, ConfigDir: configDir, UserID: userID, Password: password}); err != nil {
		_ = writeLine(rw, "EXIT 1")
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{err.Error()})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		notifyFailure(results)
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
		notifyFailure(results)
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
