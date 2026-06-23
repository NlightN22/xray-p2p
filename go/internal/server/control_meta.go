package server

import (
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

func buildControlRuntime(cfg config.Config, desired desiredServerConfig, certPath, keyPath, statePath string) (controlplane.Runtime, error) {
	controlPort := parsePortOrDefault(cfg.Server.Port, parsePortOrDefault(DefaultPort, 62022))
	trojanPort := parsePortOrDefault(cfg.Server.TrojanPort, DefaultTrojanPort)
	host := controlHost(cfg, certPath)
	tlsMeta := controlTLSMetadata(host, certPath, keyPath)
	profile, err := serverProfile(cfg.Server.Profile)
	if err != nil {
		return controlplane.Runtime{}, err
	}
	endpoint, err := tunnel.DefaultProfile(profile)
	if err != nil {
		return controlplane.Runtime{}, err
	}
	parameters := map[string]string{"tunnel_port": strconv.Itoa(trojanPort)}
	if flow := endpoint.Metadata["flow"]; flow != "" {
		parameters["flow"] = flow
	}
	sub, err := controlplane.BuildSubscription(controlplane.Subscription{
		Profile:    string(endpoint.Profile),
		Protocol:   endpoint.Protocol,
		Transport:  "tcp",
		Security:   endpoint.Security,
		Host:       host,
		Port:       trojanPort,
		ServerName: tlsMeta.ServerName,
		TLS:        tlsMeta,
		Parameters: parameters,
	}, time.Now().UTC(), time.Hour)
	if err != nil {
		return controlplane.Runtime{}, err
	}
	runtime := controlplane.Runtime{
		Endpoint: controlplane.Endpoint{
			Scheme: "https",
			Host:   host,
			Port:   controlPort,
		},
		Subscription:  sub,
		AuthUsers:     controlAuthUsers(activeServerUsers(desired.Users), time.Now().UTC()),
		RotationUsers: controlRotationUsers(desired.Users),
		TLS:           tlsMeta,
	}
	generation, err := LoadHAGeneration(statePath)
	if err != nil {
		return controlplane.Runtime{}, err
	}
	if generation.Number != 0 {
		runtime.Subscription.Topology = &controlplane.Topology{Generation: generation.Number, Group: generation.Group, Channels: generation.Channels}
		runtime.Subscription, err = controlplane.BuildSubscription(runtime.Subscription, time.Now().UTC(), time.Hour)
		if err != nil {
			return controlplane.Runtime{}, err
		}
	}
	return runtime, nil
}

func controlRotationUsers(users []trojanClient) []controlplane.RotationUser {
	out := make([]controlplane.RotationUser, 0, len(users))
	for _, user := range users {
		if user.Disabled || strings.TrimSpace(user.Email) == "" || strings.TrimSpace(user.Password) == "" {
			continue
		}
		out = append(out, controlplane.RotationUser{UserLabel: user.Email, ActiveCredential: user.Password, PreviousCredentialForRotation: user.PreviousCredentialForRotation, RotationExpiresAt: user.RotationExpiresAt, CredentialGeneration: user.CredentialGeneration})
	}
	return out
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

func controlAuthUsers(users []trojanClient, now time.Time) []controlplane.AuthUser {
	out := make([]controlplane.AuthUser, 0, len(users))
	for _, user := range users {
		label := strings.TrimSpace(user.Email)
		secret := strings.TrimSpace(user.Password)
		if label == "" || secret == "" {
			continue
		}
		out = append(out, controlplane.AuthUser{Label: label, Credential: secret})
		previous := strings.TrimSpace(user.PreviousCredentialForRotation)
		if previous != "" && !user.RotationExpiresAt.IsZero() && now.Before(user.RotationExpiresAt) {
			out = append(out, controlplane.AuthUser{Label: label, Credential: previous})
		}
	}
	return out
}
