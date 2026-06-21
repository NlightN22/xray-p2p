package identity

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// NewTunnelCredential returns a UUID credential valid for Trojan and VLESS.
func NewTunnelCredential() (string, error) {
	return newUUID("generate tunnel credential")
}

func NewRequestID() (string, error) {
	return newUUID("generate request id")
}

func NewUserID() (string, error) {
	token, err := newBase32Token(5)
	if err != nil {
		return "", fmt.Errorf("generate user id: %w", err)
	}
	return fmt.Sprintf("client-%s@xp2p.local", token), nil
}

func NewSecret(size int) (string, error) {
	if size <= 0 {
		return "", fmt.Errorf("secret size must be positive")
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func newBase32Token(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)), nil
}

func newUUID(context string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("%s: %w", context, err)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	parts := []string{
		hex.EncodeToString(buf[0:4]),
		hex.EncodeToString(buf[4:6]),
		hex.EncodeToString(buf[6:8]),
		hex.EncodeToString(buf[8:10]),
		hex.EncodeToString(buf[10:16]),
	}
	return strings.Join(parts, "-"), nil
}
