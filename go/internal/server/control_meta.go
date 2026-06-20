package server

import (
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
)

func buildControlRuntime(cfg config.Config, desired desiredServerConfig, certPath, keyPath string) (controlplane.Runtime, error) {
	controlPort := parsePortOrDefault(cfg.Server.Port, parsePortOrDefault(DefaultPort, 62022))
	trojanPort := parsePortOrDefault(cfg.Server.TrojanPort, DefaultTrojanPort)
	host := controlHost(cfg, certPath)
	tlsMeta := controlTLSMetadata(host, certPath, keyPath)
	sub, err := controlplane.BuildSubscription(controlplane.Subscription{
		Profile:    "trojan-tls",
		Protocol:   "trojan",
		Transport:  "tcp",
		Security:   subscriptionSecurity(certPath, keyPath),
		Host:       host,
		Port:       trojanPort,
		ServerName: tlsMeta.ServerName,
		TLS:        tlsMeta,
		Parameters: map[string]string{
			"tunnel_port": strconv.Itoa(trojanPort),
		},
	}, time.Now().UTC(), time.Hour)
	if err != nil {
		return controlplane.Runtime{}, err
	}
	return controlplane.Runtime{
		Endpoint: controlplane.Endpoint{
			Scheme: "https",
			Host:   host,
			Port:   controlPort,
		},
		Subscription: sub,
		AuthUsers:    controlAuthUsers(activeServerUsers(desired.Users)),
		TLS:          tlsMeta,
	}, nil
}

func controlHost(cfg config.Config, certPath string) string {
	host := strings.TrimSpace(cfg.Server.Host)
	if host != "" {
		return host
	}
	if certPath != "" {
		if candidates, err := resolveLinkHostFromCertificate(certPath); err == nil && len(candidates) > 0 {
			return candidates[0]
		}
	}
	return "localhost"
}

func controlTLSMetadata(host, certPath, keyPath string) controlplane.TLSMetadata {
	if certPath == "" || keyPath == "" {
		return controlplane.TLSMetadata{ClientMayAllowInsecure: true}
	}
	meta := controlplane.TLSMetadata{
		ServerName:           host,
		CertificatePath:      certPath,
		VerifyPeerCertByName: host,
	}
	if selfSigned, err := isSelfSignedCertificatePath(certPath); err == nil && selfSigned {
		meta.SelfSigned = true
		if fp, fpErr := certificateFingerprintSHA256(certPath); fpErr == nil {
			meta.PinnedPeerCertSHA256 = fp
		}
	}
	return meta
}

func subscriptionSecurity(certPath, keyPath string) string {
	if certPath != "" && keyPath != "" {
		return "tls"
	}
	return "none"
}

func controlAuthUsers(users []trojanClient) []controlplane.AuthUser {
	out := make([]controlplane.AuthUser, 0, len(users))
	for _, user := range users {
		label := strings.TrimSpace(user.Email)
		secret := strings.TrimSpace(user.Password)
		if label == "" || secret == "" {
			continue
		}
		out = append(out, controlplane.AuthUser{Label: label, Credential: secret})
	}
	return out
}
