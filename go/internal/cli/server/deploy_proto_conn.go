package servercmd

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
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
