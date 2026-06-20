package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
)

func postHeartbeat(ctx context.Context, host string, port int, endpoint clientEndpointRecord, secret string, payload heartbeat.Payload, timeout time.Duration) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := "https://" + net.JoinHostPort(host, fmt.Sprintf("%d", port)) + controlplane.PathHeartbeat
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		if err := controlplane.ApplyHeaders(req, endpoint.User, secret, payloadNonce(payload), body, time.Now().UTC()); err != nil {
			return err
		}
	}
	resp, err := controlHTTPClient(endpoint, timeout).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("control heartbeat failed: %s", resp.Status)
	}
	return nil
}

func controlAuthMap(users []controlplane.AuthUser) map[string]string {
	out := make(map[string]string, len(users))
	for _, user := range users {
		label := strings.TrimSpace(user.Label)
		secret := strings.TrimSpace(user.Credential)
		if label != "" && secret != "" {
			out[label] = secret
		}
	}
	return out
}

func controlHTTPClient(endpoint clientEndpointRecord, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: controlTLSConfig(endpoint),
			DialContext:     (&net.Dialer{Timeout: timeout}).DialContext,
		},
	}
}

func controlTLSConfig(endpoint clientEndpointRecord) *tls.Config {
	pin := strings.TrimSpace(endpoint.PinnedPeerCertSHA256)
	cfg := &tls.Config{
		ServerName:         strings.TrimSpace(endpoint.ServerName),
		InsecureSkipVerify: endpoint.AllowInsecure || pin != "",
	}
	if pin != "" {
		cfg.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return verifyControlPinnedPeerCertificate(rawCerts, pin)
		}
	}
	return cfg
}

func verifyControlPinnedPeerCertificate(rawCerts [][]byte, pin string) error {
	return verifyPinnedPeerCertificate(rawCerts, pin)
}

func payloadNonce(payload heartbeat.Payload) string {
	nonce := strings.TrimSpace(payload.Tag) + "-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if strings.TrimSpace(payload.Tag) == "" {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return nonce
}

func controlNonce() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func verifyPinnedPeerCertificate(rawCerts [][]byte, pin string) error {
	pin = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(pin), ":", ""))
	if pin == "" {
		return errors.New("peer certificate pin is empty")
	}
	if len(rawCerts) == 0 {
		return errors.New("peer certificate is missing")
	}
	sum := sha256.Sum256(rawCerts[0])
	got := hex.EncodeToString(sum[:])
	if got != pin {
		return errors.New("peer certificate pin mismatch")
	}
	return nil
}
