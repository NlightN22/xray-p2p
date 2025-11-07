package link

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
)

const (
	Scheme      = "xp2p+deploy"
	Version     = "2"
	aad         = "XP2PDEPLOY|v=2"
	keySize     = 32
	nonceSize   = 12
	defaultPort = "62025"
)

// EncryptedLink holds encrypted manifest data and associated metadata.
type EncryptedLink struct {
	Host          string
	Port          string
	Key           string
	Nonce         string
	Ciphertext    []byte
	CiphertextB64 string
	ExpiresAt     int64
	Manifest      spec.Manifest
}

// Build constructs an encrypted deploy link for the provided manifest.
func Build(remoteHost, deployPort string, manifest spec.Manifest, ttl time.Duration) (string, EncryptedLink, error) {
	remoteHost = strings.TrimSpace(remoteHost)
	if strings.TrimSpace(manifest.Host) == "" {
		manifest.Host = remoteHost
	}
	manifest = spec.Normalize(manifest)
	if manifest.Host == "" {
		return "", EncryptedLink{}, spec.ErrHostEmpty
	}
	if manifest.ExpiresAt == 0 {
		manifest.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	if err := spec.Validate(manifest); err != nil {
		return "", EncryptedLink{}, err
	}

	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return "", EncryptedLink{}, fmt.Errorf("generate key: %w", err)
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", EncryptedLink{}, fmt.Errorf("generate nonce: %w", err)
	}

	payload, err := spec.Marshal(manifest)
	if err != nil {
		return "", EncryptedLink{}, err
	}

	ct, err := encrypt(key, nonce, payload)
	if err != nil {
		return "", EncryptedLink{}, err
	}

	enc := EncryptedLink{
		Host:          manifest.Host,
		Port:          normalizePort(deployPort),
		Key:           base64.RawURLEncoding.EncodeToString(key),
		Nonce:         base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext:    ct,
		CiphertextB64: base64.RawURLEncoding.EncodeToString(ct),
		ExpiresAt:     manifest.ExpiresAt,
		Manifest:      manifest,
	}

	return enc.URL(), enc, nil
}

// Parse parses a deploy link string and decrypts the embedded manifest.
func Parse(raw string) (EncryptedLink, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return EncryptedLink{}, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return EncryptedLink{}, err
	}
	if !strings.EqualFold(u.Scheme, Scheme) {
		return EncryptedLink{}, fmt.Errorf("invalid scheme %q", u.Scheme)
	}

	linkHost := strings.TrimSpace(u.Hostname())
	port := normalizePort(u.Port())

	q := u.Query()
	if v := strings.TrimSpace(q.Get("v")); v != Version {
		return EncryptedLink{}, fmt.Errorf("unsupported deploy link version %q", v)
	}

	keyB64 := strings.TrimSpace(q.Get("k"))
	cipherB64 := strings.TrimSpace(q.Get("ct"))
	nonceB64 := strings.TrimSpace(q.Get("n"))
	if keyB64 == "" || cipherB64 == "" || nonceB64 == "" {
		return EncryptedLink{}, fmt.Errorf("deploy link missing required parameters")
	}

	var expUnix int64
	if rawExp := strings.TrimSpace(q.Get("exp")); rawExp != "" {
		value, err := strconv.ParseInt(rawExp, 10, 64)
		if err != nil {
			return EncryptedLink{}, fmt.Errorf("invalid exp value %q", rawExp)
		}
		expUnix = value
	}

	key, err := base64.RawURLEncoding.DecodeString(keyB64)
	if err != nil {
		return EncryptedLink{}, fmt.Errorf("decode key: %w", err)
	}
	nonce, err := base64.RawURLEncoding.DecodeString(nonceB64)
	if err != nil {
		return EncryptedLink{}, fmt.Errorf("decode nonce: %w", err)
	}
	ct, err := base64.RawURLEncoding.DecodeString(cipherB64)
	if err != nil {
		return EncryptedLink{}, fmt.Errorf("decode ciphertext: %w", err)
	}

	plain, err := decrypt(key, nonce, ct)
	if err != nil {
		return EncryptedLink{}, fmt.Errorf("decrypt manifest: %w", err)
	}
	manifest, err := spec.Unmarshal(plain)
	if err != nil {
		return EncryptedLink{}, err
	}

	manifestHost := strings.TrimSpace(manifest.Host)
	if manifestHost == "" {
		manifestHost = linkHost
	} else if linkHost != "" && !strings.EqualFold(linkHost, manifestHost) {
		return EncryptedLink{}, fmt.Errorf("link host %q mismatches manifest host %q", linkHost, manifestHost)
	}
	if manifestHost == "" {
		return EncryptedLink{}, spec.ErrHostEmpty
	}
	manifest.Host = manifestHost
	if expUnix != 0 {
		manifest.ExpiresAt = expUnix
	}

	if err := spec.Validate(manifest); err != nil {
		return EncryptedLink{}, err
	}

	return EncryptedLink{
		Host:          manifest.Host,
		Port:          port,
		Key:           keyB64,
		Nonce:         nonceB64,
		Ciphertext:    ct,
		CiphertextB64: cipherB64,
		ExpiresAt:     manifest.ExpiresAt,
		Manifest:      manifest,
	}, nil
}

// URL returns the full deploy link string including the encryption key.
func (e EncryptedLink) URL() string {
	host := e.Host
	if host == "" {
		host = "localhost"
	}
	port := normalizePort(e.Port)
	return fmt.Sprintf("%s://%s:%s?v=%s&k=%s&ct=%s&n=%s&exp=%d", Scheme, host, port, Version, e.Key, e.CiphertextB64, e.Nonce, e.ExpiresAt)
}

// RedactedURL returns the deploy link string without embedding the key.
func RedactedURL(host, port, nonce string, exp int64, cipherB64 string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "localhost"
	}
	port = normalizePort(port)
	return fmt.Sprintf("%s://%s:%s?v=%s&ct=%s&n=%s&exp=%d", Scheme, host, port, Version, cipherB64, nonce, exp)
}

func encrypt(key, nonce, payload []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, payload, []byte(aad)), nil
}

func decrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, []byte(aad))
}

func normalizePort(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return defaultPort
	}
	return port
}
