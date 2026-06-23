package controlplane

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func BuildSubscription(sub Subscription, now time.Time, ttl time.Duration) (Subscription, error) {
	sub.TLS = ClientVisibleTLSMetadata(sub.TLS)
	sub.IssuedAt = now.UTC()
	if ttl <= 0 {
		ttl = time.Hour
	}
	sub.ValidUntil = sub.IssuedAt.Add(ttl)
	gen, err := Generation(sub)
	if err != nil {
		return Subscription{}, err
	}
	sub.Generation = gen
	return sub, nil
}

func Generation(sub Subscription) (string, error) {
	sub.TLS = ClientVisibleTLSMetadata(sub.TLS)
	canonical := struct {
		Profile    string            `json:"profile"`
		Protocol   string            `json:"protocol"`
		Transport  string            `json:"transport"`
		Security   string            `json:"security"`
		Host       string            `json:"host"`
		Port       int               `json:"port"`
		ServerName string            `json:"server_name,omitempty"`
		TLS        TLSMetadata       `json:"tls,omitempty"`
		Parameters map[string]string `json:"parameters,omitempty"`
		Topology   *Topology         `json:"topology,omitempty"`
	}{
		Profile:    strings.TrimSpace(sub.Profile),
		Protocol:   strings.TrimSpace(sub.Protocol),
		Transport:  strings.TrimSpace(sub.Transport),
		Security:   strings.TrimSpace(sub.Security),
		Host:       strings.TrimSpace(sub.Host),
		Port:       sub.Port,
		ServerName: strings.TrimSpace(sub.ServerName),
		TLS:        sub.TLS,
		Parameters: sub.Parameters,
		Topology:   sub.Topology,
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("encode subscription generation payload: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func ClientVisibleTLSMetadata(meta TLSMetadata) TLSMetadata {
	return TLSMetadata{
		ServerName:             strings.TrimSpace(meta.ServerName),
		PinnedPeerCertSHA256:   strings.TrimSpace(meta.PinnedPeerCertSHA256),
		VerifyPeerCertByName:   strings.TrimSpace(meta.VerifyPeerCertByName),
		ClientMayAllowInsecure: meta.ClientMayAllowInsecure,
	}
}
