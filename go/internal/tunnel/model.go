// Package tunnel defines protocol-neutral tunnel configuration.
package tunnel

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Profile string

const (
	ProfileTrojanTLS      Profile = "trojan-tls"
	ProfileVLESSTLSVision Profile = "vless-tls-vision"
)

type User struct {
	UserLabel                     string            `json:"user_label" toml:"user_label"`
	ActiveCredential              string            `json:"active_credential" toml:"active_credential"`
	PreviousCredentialForRotation string            `json:"previous_credential_for_rotation,omitempty" toml:"previous_credential_for_rotation,omitempty"`
	RotationExpiresAt             time.Time         `json:"rotation_expires_at,omitempty" toml:"rotation_expires_at,omitempty"`
	CredentialGeneration          int               `json:"credential_generation" toml:"credential_generation"`
	Credential                    string            `json:"credential,omitempty" toml:"credential,omitempty"`
	Disabled                      bool              `json:"disabled,omitempty" toml:"disabled,omitempty"`
	Metadata                      map[string]string `json:"metadata,omitempty" toml:"metadata,omitempty"`
}

// IsUUIDCredential reports whether a credential can be used as a VLESS id.
func IsUUIDCredential(credential string) bool {
	value := strings.TrimSpace(credential)
	if len(value) != 36 {
		return false
	}
	for _, index := range []int{8, 13, 18, 23} {
		if value[index] != '-' {
			return false
		}
	}
	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil
}

// ValidateVLESSCredential is shared by VLESS links and Xray profile codecs.
func ValidateVLESSCredential(credential string) error {
	if !IsUUIDCredential(credential) {
		return errors.New("VLESS credential must be a UUID")
	}
	return nil
}

type TLSMetadata struct {
	ALPN                 []string `json:"alpn,omitempty" toml:"alpn,omitempty"`
	AllowInsecure        bool     `json:"allow_insecure,omitempty" toml:"allow_insecure,omitempty"`
	PinnedPeerCertSHA256 string   `json:"pinned_peer_cert_sha256,omitempty" toml:"pinned_peer_cert_sha256,omitempty"`
	VerifyPeerCertByName string   `json:"verify_peer_cert_by_name,omitempty" toml:"verify_peer_cert_by_name,omitempty"`
}

type Endpoint struct {
	Host       string            `json:"host" toml:"host"`
	Port       int               `json:"port" toml:"port"`
	Profile    Profile           `json:"profile" toml:"profile"`
	Protocol   string            `json:"protocol" toml:"protocol"`
	Transport  string            `json:"transport" toml:"transport"`
	Security   string            `json:"security" toml:"security"`
	ServerName string            `json:"server_name,omitempty" toml:"server_name,omitempty"`
	Generation int               `json:"generation,omitempty" toml:"generation,omitempty"`
	TLS        TLSMetadata       `json:"tls,omitempty" toml:"tls,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty" toml:"metadata,omitempty"`
}

func DefaultProfile(profile Profile) (Endpoint, error) {
	switch profile {
	case "", ProfileTrojanTLS:
		return Endpoint{Profile: ProfileTrojanTLS, Protocol: "trojan", Transport: "tcp", Security: "tls"}, nil
	case ProfileVLESSTLSVision:
		return Endpoint{Profile: ProfileVLESSTLSVision, Protocol: "vless", Transport: "tcp", Security: "tls", Metadata: map[string]string{"flow": "xtls-rprx-vision"}}, nil
	default:
		return Endpoint{}, fmt.Errorf("unsupported tunnel profile %q", profile)
	}
}

// Normalize applies compatibility defaults to records written before profiles existed.
func Normalize(endpoint Endpoint) (Endpoint, error) {
	defaults, err := DefaultProfile(endpoint.Profile)
	if err != nil {
		return Endpoint{}, err
	}
	if strings.TrimSpace(endpoint.Protocol) == "" {
		endpoint.Protocol = defaults.Protocol
	}
	if strings.TrimSpace(endpoint.Transport) == "" {
		endpoint.Transport = defaults.Transport
	}
	if strings.TrimSpace(endpoint.Security) == "" {
		endpoint.Security = defaults.Security
	}
	endpoint.Profile = defaults.Profile
	if !strings.EqualFold(endpoint.Security, "tls") && !strings.EqualFold(endpoint.Security, "reality") {
		return Endpoint{}, fmt.Errorf("managed tunnel profile requires TLS security")
	}
	if !strings.EqualFold(endpoint.Protocol, defaults.Protocol) {
		return Endpoint{}, fmt.Errorf("profile %q requires protocol %q", endpoint.Profile, defaults.Protocol)
	}
	return endpoint, nil
}

// NewCredential generates a UUID credential valid for both Trojan and VLESS.
func NewCredential() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate credential: %w", err)
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", data[:4], data[4:6], data[6:8], data[8:10], data[10:]), nil
}
