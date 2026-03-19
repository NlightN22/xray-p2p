package servercmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

const (
	serverDeployIOTimeout         = 60 * time.Second
	serverDeployCompletionTimeout = 5 * time.Minute
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
	logging.Info("xp2p server deploy: received encrypted manifest", "bytes", len(buf), "ciphertext_b64", cipherB64)

	if strings.TrimSpace(s.Expected.Link) == "" {
		_ = writeLine(rw, "ERR deploy link not configured")
		notifyFailure(results)
		return
	}

	manifest, err := deploylink.Decrypt(s.Expected.Link, buf)
	if err != nil {
		_ = writeLine(rw, "ERR unauthorized")
		notifyFailure(results)
		return
	}

	if manifest.ExpiresAt > 0 && time.Now().Unix() > manifest.ExpiresAt {
		_ = writeLine(rw, "ERR link expired")
		notifyFailure(results)
		return
	}

	canonicalLink, err := deploylink.CanonicalLink(manifest)
	if err != nil {
		_ = writeLine(rw, "ERR invalid manifest")
		notifyFailure(results)
		return
	}
	if canonicalLink != s.Expected.Link {
		_ = writeLine(rw, "ERR unauthorized")
		notifyFailure(results)
		return
	}
	logging.Info("xp2p server deploy: manifest decrypted", "host", manifest.Host, "install_dir", manifest.InstallDir, "trojan_port", manifest.TrojanPort, "user", manifest.TrojanUser, "expires_at", manifest.ExpiresAt)

	s.proceedInstall(ctx, conn, rw, results, manifest)
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

func (s *deployServer) proceedInstall(ctx context.Context, conn net.Conn, rw *bufio.ReadWriter, results chan<- runSignal, man spec.Manifest) {
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
	if err := validateWindowsDeployInstallDir(installDir); err != nil {
		_ = writeLine(rw, "ERR "+err.Error())
		notifyFailure(results)
		return
	}
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

	installed, err := clishared.InstallPresent(clishared.InstallRoleServer, installDir, configDir)
	if err != nil {
		_ = writeLine(rw, "ERR "+err.Error())
		notifyFailure(results)
		return
	}
	if installed {
		required := []string{
			"inbounds.json",
			"outbounds.json",
			"routing.json",
			"logs.json",
		}
		for _, name := range required {
			path := filepath.Join(configDir, name)
			if _, err := os.Stat(path); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					_ = writeLine(rw, "ERR xp2p: server install incomplete: missing "+name)
					notifyFailure(results)
					return
				}
				_ = writeLine(rw, "ERR "+err.Error())
				notifyFailure(results)
				return
			}
		}
	}

	inst := server.InstallOptions{
		InstallDir:            installDir,
		ConfigDir:             configDir,
		Port:                  port,
		CertificateStore:      strings.TrimSpace(s.Cfg.Server.CertificateStore),
		CertificateFile:       strings.TrimSpace(s.Cfg.Server.CertificateFile),
		KeyFile:               strings.TrimSpace(s.Cfg.Server.KeyFile),
		Host:                  host,
		Force:                 true,
		RelaxedPathValidation: true,
		TunEnabled:            false,
		TunEnabledSet:         true,
		TunName:               s.Cfg.Server.TunName,
		TunMTU:                s.Cfg.Server.TunMTU,
		TunAddr:               s.Cfg.Server.TunAddr,
	}
	if installed {
		logging.Info("xp2p server deploy: installation detected, running in append mode", "config", config.ConfigPath(layout.ServerConfigFileName))
		goto installDone
	}
	if err := server.Install(ctx, inst); err != nil {
		if server.IsCertificateValidationError(err) {
			logging.Warn("xp2p server deploy: certificate validation failed, using self-signed", "err", err)
			inst.CertificateStore = ""
			inst.CertificateFile = ""
			inst.KeyFile = ""
			s.Cfg.Server.CertificateStore = ""
			s.Cfg.Server.CertificateFile = ""
			s.Cfg.Server.KeyFile = ""
			_ = os.Unsetenv("XP2P_SERVER_CERTIFICATE")
			_ = os.Unsetenv("XP2P_SERVER_KEY")
			if _, updateErr := config.ClearServerCertificateOverrides(""); updateErr != nil {
				logging.Warn("xp2p server deploy: failed to clear certificate overrides", "err", updateErr)
			}
			if retryErr := server.Install(ctx, inst); retryErr == nil {
				goto installDone
			} else {
				err = retryErr
			}
		}
		_ = writeLine(rw, "EXIT 1")
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{err.Error()})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		notifyFailure(results)
		return
	}
installDone:
	if !installed {
		if _, err := config.UpdateServerTrojanPortBestEffort("", port); err != nil {
			_ = writeLine(rw, "ERR "+err.Error())
			notifyFailure(results)
			return
		}
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
	if err := clishared.ValidateRFC3986Unreserved(password); err != nil {
		_ = writeLine(rw, "ERR invalid password")
		notifyFailure(results)
		return
	}

	if err := server.AddUser(ctx, server.AddUserOptions{InstallDir: installDir, ConfigDir: configDir, UserID: userID, Password: password, Host: host, Force: true}); err != nil {
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
	s.waitForCompletion(conn, rw, results)
}

func (s *deployServer) waitForCompletion(conn net.Conn, rw *bufio.ReadWriter, results chan<- runSignal) {
	if conn != nil {
		_ = conn.SetDeadline(time.Now().Add(serverDeployCompletionTimeout))
	}
	line, err := readLine(rw)
	status := ""
	if err != nil {
		logging.Warn("xp2p server deploy: completion wait failed", "err", err)
		status = "error"
	} else if !strings.HasPrefix(line, "COMPLETE") {
		logging.Warn("xp2p server deploy: unexpected completion signal", "line", line)
		status = strings.TrimSpace(line)
	} else {
		status = strings.TrimSpace(strings.TrimPrefix(line, "COMPLETE"))
		if ackErr := writeLine(rw, "BYE"); ackErr != nil {
			logging.Warn("xp2p server deploy: completion ack failed", "err", ackErr)
		}
	}
	if results != nil {
		results <- runSignal{completed: true, status: status}
	}
}

func validateWindowsDeployInstallDir(installDir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	installDir = strings.TrimSpace(installDir)
	if installDir == "" {
		return nil
	}
	if strings.HasPrefix(installDir, "/") && !strings.HasPrefix(installDir, "//") {
		return fmt.Errorf("invalid install_dir for Windows: %q", installDir)
	}
	return nil
}
