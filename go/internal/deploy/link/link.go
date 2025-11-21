package link

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
)

const (
	// Scheme defines the link prefix presented to users (canonical trojan link).
	Scheme    = "trojan"
	aad       = "XP2PDEPLOY|v=2"
	nonceSize = 12
	keySize   = 32
)

// EncryptedLink keeps the canonical trojan link together with the encrypted manifest.
type EncryptedLink struct {
	Link       string
	Host       string
	Port       string
	Ciphertext []byte
	ExpiresAt  int64
	Manifest   spec.Manifest
}

// Build constructs a trojan link from the manifest, derives an encryption key from it,
// and returns the ciphertext together with the canonical link string.
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

	linkURL, err := CanonicalLink(manifest)
	if err != nil {
		return "", EncryptedLink{}, err
	}

	payload, err := spec.Marshal(manifest)
	if err != nil {
		return "", EncryptedLink{}, err
	}

	key, nonce := deriveKeyNonce(linkURL)
	ct, err := encrypt(key, nonce, payload)
	if err != nil {
		return "", EncryptedLink{}, err
	}

	enc := EncryptedLink{
		Link:       linkURL,
		Host:       manifest.Host,
		Port:       manifest.TrojanPort,
		Ciphertext: ct,
		ExpiresAt:  manifest.ExpiresAt,
		Manifest:   manifest,
	}

	// deployPort kept to avoid signature churn; it is unused in v3 links.
	_ = deployPort

	return linkURL, enc, nil
}

// Parse validates a trojan deploy link and normalizes it for consistent key derivation.
func Parse(raw string) (EncryptedLink, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return EncryptedLink{}, nil
	}

	canonical, host, port, exp, err := normalizeTrojanLink(raw)
	if err != nil {
		return EncryptedLink{}, err
	}

	return EncryptedLink{
		Link:      canonical,
		Host:      host,
		Port:      port,
		ExpiresAt: exp,
	}, nil
}

// CanonicalLink renders the manifest as a deterministic trojan link string.
func CanonicalLink(manifest spec.Manifest) (string, error) {
	host := strings.TrimSpace(manifest.Host)
	if host == "" {
		return "", spec.ErrHostEmpty
	}
	port := strings.TrimSpace(manifest.TrojanPort)
	if port == "" {
		return "", fmt.Errorf("xp2p: deploy manifest missing trojan port")
	}
	password := strings.TrimSpace(manifest.TrojanPassword)
	if password == "" {
		return "", fmt.Errorf("xp2p: deploy manifest missing trojan password")
	}
	user := strings.TrimSpace(manifest.TrojanUser)
	if user == "" {
		return "", fmt.Errorf("xp2p: deploy manifest missing trojan user")
	}

	params := url.Values{}
	params.Set("security", "tls")
	params.Set("sni", host)
	if manifest.InstallDir != "" {
		params.Set("install_dir", manifest.InstallDir)
	}
	if manifest.Version > 0 {
		params.Set("deploy_version", strconv.Itoa(manifest.Version))
	}
	if manifest.ExpiresAt != 0 {
		params.Set("exp", strconv.FormatInt(manifest.ExpiresAt, 10))
	}

	return renderTrojanURL(host, port, password, user, params), nil
}

// Decrypt decrypts an encrypted manifest using the canonical trojan link as key material.
func Decrypt(link string, ciphertext []byte) (spec.Manifest, error) {
	if strings.TrimSpace(link) == "" {
		return spec.Manifest{}, fmt.Errorf("xp2p: link is empty")
	}
	if len(ciphertext) == 0 {
		return spec.Manifest{}, fmt.Errorf("xp2p: ciphertext is empty")
	}

	key, nonce := deriveKeyNonce(link)
	plain, err := decrypt(key, nonce, ciphertext)
	if err != nil {
		return spec.Manifest{}, err
	}
	return spec.Unmarshal(plain)
}

func normalizeTrojanLink(raw string) (string, string, string, int64, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", 0, err
	}
	if !strings.EqualFold(u.Scheme, Scheme) {
		return "", "", "", 0, fmt.Errorf("invalid scheme %q", u.Scheme)
	}

	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return "", "", "", 0, fmt.Errorf("trojan link missing host")
	}

	port := strings.TrimSpace(u.Port())
	if port == "" {
		return "", "", "", 0, fmt.Errorf("trojan link missing port")
	}

	if u.User == nil {
		return "", "", "", 0, fmt.Errorf("trojan link missing password")
	}
	password := strings.TrimSpace(u.User.String())
	if pwd, hasPwd := u.User.Password(); hasPwd {
		password = strings.TrimSpace(pwd)
	}
	if password == "" {
		return "", "", "", 0, fmt.Errorf("trojan link missing password")
	}

	userFragment := strings.TrimSpace(u.Fragment)
	query := u.Query()

	var expUnix int64
	if rawExp := strings.TrimSpace(query.Get("exp")); rawExp != "" {
		value, err := strconv.ParseInt(rawExp, 10, 64)
		if err != nil {
			return "", "", "", 0, fmt.Errorf("invalid exp value %q", rawExp)
		}
		expUnix = value
	}

	canonical := renderTrojanURL(host, port, password, userFragment, query)
	return canonical, host, port, expUnix, nil
}

func renderTrojanURL(host, port, password, fragment string, query url.Values) string {
	var builder strings.Builder
	builder.Grow(len(host) + len(port) + len(password) + 32)
	builder.WriteString(Scheme)
	builder.WriteString("://")
	builder.WriteString(url.PathEscape(password))
	builder.WriteString("@")
	builder.WriteString(host)
	builder.WriteString(":")
	builder.WriteString(port)

	if encoded := query.Encode(); encoded != "" {
		builder.WriteString("?")
		builder.WriteString(encoded)
	}
	if fragment != "" {
		builder.WriteString("#")
		builder.WriteString(url.PathEscape(fragment))
	}
	return builder.String()
}

func deriveKeyNonce(link string) ([]byte, []byte) {
	sum := sha512.Sum512([]byte(link))
	key := make([]byte, keySize)
	copy(key, sum[:keySize])

	nonce := make([]byte, nonceSize)
	copy(nonce, sum[keySize:keySize+nonceSize])
	return key, nonce
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
