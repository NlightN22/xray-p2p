package clientcmd

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func normalizeServerPort(cfg config.Config, flagPort string) string {
	if strings.TrimSpace(flagPort) != "" {
		return strings.TrimSpace(flagPort)
	}
	if cfgPort := strings.TrimSpace(cfg.Server.Port); cfgPort != "" && cfgPort != server.DefaultPort {
		return cfgPort
	}
	return fmt.Sprintf("%d", server.DefaultTrojanPort)
}

func generateSecret(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func generateDeployToken() (string, error) {
	// 10 bytes -> 16 base32 chars; lower-case, no padding
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	return strings.ToLower(enc), nil
}

func generateHMACKey(size int) (string, error) {
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

// crypto helpers for v2 encrypted deploy links

func generateAESKey() (string, []byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", nil, err
	}
	return base64.RawURLEncoding.EncodeToString(key), key, nil
}

func generateNonce() (string, []byte, error) {
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", nil, err
	}
	return base64.RawURLEncoding.EncodeToString(nonce), nonce, nil
}

func encryptManifestAESGCM(key []byte, nonce []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	aad := []byte("XP2PDEPLOY|v=2")
	return aead.Seal(nil, nonce, plaintext, aad), nil
}

func nowPlusMinutes(mins int) int64 {
	return time.Now().Add(time.Duration(mins) * time.Minute).Unix()
}
