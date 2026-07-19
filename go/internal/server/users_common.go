package server

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identity"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

const (
	serverTrojanUsersKey = "trojan_users"
	serverUsersKey       = "users"
)

type trojanClient struct {
	Email                         string    `json:"email" toml:"email"`
	Password                      string    `json:"password" toml:"password"`
	PreviousCredentialForRotation string    `json:"-"`
	RotationExpiresAt             time.Time `json:"-"`
	CredentialGeneration          int       `json:"-"`
	Disabled                      bool      `json:"disabled,omitempty" toml:"disabled,omitempty"`
	ManagedByIdentity             bool      `json:"-"`
}

// LegacyTrojanUser exposes the compatibility input contract to tooling.
type LegacyTrojanUser = trojanClient

func decodeServerTrojanUsers(doc map[string]any) ([]trojanClient, error) {
	if raw := doc[serverUsersKey]; raw != nil {
		buf, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("encode server users: %w", err)
		}
		var users []tunnel.User
		if err := json.Unmarshal(buf, &users); err != nil {
			return nil, fmt.Errorf("decode server users: %w", err)
		}
		result := make([]trojanClient, 0, len(users))
		for _, user := range users {
			managed := user.Metadata["managed_by"] == "identity"
			if identity.IsManagedUserLabel(user.UserLabel) && !managed {
				return nil, fmt.Errorf("manual user label with reserved idp- prefix is not allowed")
			}
			active := strings.TrimSpace(tunnel.ActiveCredential(user))
			result = append(result, trojanClient{Email: user.UserLabel, Password: active, PreviousCredentialForRotation: user.PreviousCredentialForRotation, RotationExpiresAt: user.RotationExpiresAt, CredentialGeneration: user.CredentialGeneration, Disabled: user.Disabled, ManagedByIdentity: managed})
		}
		return result, nil
	}
	raw := doc[serverTrojanUsersKey]
	if raw == nil {
		return []trojanClient{}, nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode server users: %w", err)
	}
	var users []trojanClient
	if err := json.Unmarshal(buf, &users); err != nil {
		return nil, fmt.Errorf("decode server users: %w", err)
	}
	if users == nil {
		users = []trojanClient{}
	}
	for idx := range users {
		user := users[idx]
		record, _, err := tunnel.NormalizeRecord(tunnel.LegacyRecord{
			UserLabel: user.Email,
			Password:  user.Password,
			Disabled:  user.Disabled,
		})
		if err != nil {
			return nil, fmt.Errorf("normalize legacy server user %q: %w", user.Email, err)
		}
		users[idx] = trojanClient{Email: record.User.UserLabel, Password: tunnel.ActiveCredential(record.User), CredentialGeneration: 1, Disabled: record.User.Disabled}
		if identity.IsManagedUserLabel(users[idx].Email) {
			return nil, fmt.Errorf("manual user label with reserved idp- prefix is not allowed")
		}
	}
	return users, nil
}

func setServerUsers(doc map[string]any, users []trojanClient) {
	stored := make([]tunnel.User, 0, len(users))
	for _, user := range users {
		generation := user.CredentialGeneration
		if generation == 0 {
			generation = 1
		}
		metadata := map[string]string(nil)
		if user.ManagedByIdentity {
			metadata = map[string]string{"managed_by": "identity"}
		}
		stored = append(stored, tunnel.User{UserLabel: user.Email, ActiveCredential: user.Password, PreviousCredentialForRotation: user.PreviousCredentialForRotation, RotationExpiresAt: user.RotationExpiresAt, CredentialGeneration: generation, Disabled: user.Disabled, Metadata: metadata})
	}
	doc[serverUsersKey] = stored
	doc[serverTrojanUsersKey] = nil
}

func saveServerTrojanUsers(configPath string, users []trojanClient) error {
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return err
	}
	setServerUsers(doc, users)
	return writeServerStateDoc(configPath, doc)
}

func clientsToInterfaces(clients []trojanClient) []any {
	result := make([]any, 0, len(clients))
	now := time.Now().UTC()
	for _, client := range clients {
		if client.Disabled {
			continue
		}
		result = append(result, trojanClientInterface(client, strings.TrimSpace(client.Password), true))
		previous := strings.TrimSpace(client.PreviousCredentialForRotation)
		if previous != "" && !client.RotationExpiresAt.IsZero() && now.Before(client.RotationExpiresAt) {
			result = append(result, trojanClientInterface(trojanClient{Email: previousTrojanEmail(client)}, previous, true))
		}
	}
	return result
}

func previousTrojanEmail(client trojanClient) string {
	email := strings.TrimSpace(client.Email)
	if email == "" {
		return ""
	}
	return fmt.Sprintf("%s.previous", email)
}

func trojanClientInterface(client trojanClient, password string, includeEmail bool) map[string]any {
	entry := map[string]any{
		"password": password,
	}
	if includeEmail && strings.TrimSpace(client.Email) != "" {
		entry["email"] = strings.TrimSpace(client.Email)
	}
	return entry
}

func buildTrojanLink(host string, port int, password, label string, tlsEnabled bool, pinnedPeerCertSHA256, verifyPeerCertByName string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", errors.New("host is required to build connection link")
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return "", errors.New("password is required to build connection link")
	}
	endpoint, err := tunnel.DefaultProfile(tunnel.ProfileTrojanTLS)
	if err != nil {
		return "", err
	}
	endpoint.Host = host
	endpoint.Port = port
	if tlsEnabled {
		endpoint.ServerName = host
		endpoint.TLS = tunnel.TLSMetadata{
			PinnedPeerCertSHA256: strings.TrimSpace(pinnedPeerCertSHA256),
			VerifyPeerCertByName: strings.TrimSpace(verifyPeerCertByName),
		}
	} else {
		endpoint.Security = "none"
	}
	return tunnel.RenderLink(tunnel.Link{
		Endpoint: endpoint,
		User:     tunnel.User{UserLabel: label, Credential: password},
	})
}

func buildVLESSLink(host string, port int, credential, label string, tls tunnel.TLSMetadata) (string, error) {
	if err := tunnel.ValidateVLESSCredential(credential); err != nil {
		return "", err
	}
	endpoint, err := tunnel.DefaultProfile(tunnel.ProfileVLESSTLSVision)
	if err != nil {
		return "", err
	}
	endpoint.Host = host
	endpoint.Port = port
	endpoint.ServerName = host
	endpoint.TLS = tls
	return tunnel.RenderLink(tunnel.Link{
		Endpoint: endpoint,
		User:     tunnel.User{UserLabel: label, Credential: credential},
	})
}

func resolveLinkHostFromCertificate(certPath string) ([]string, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read certificate %s: %w", certPath, err)
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("decode certificate %s: invalid PEM data", certPath)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate %s: %w", certPath, err)
	}

	candidates := make([]string, 0, len(cert.DNSNames)+len(cert.IPAddresses)+1)
	seen := make(map[string]struct{})
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	for _, name := range cert.DNSNames {
		add(name)
	}
	for _, ip := range cert.IPAddresses {
		if ip == nil {
			continue
		}
		add(ip.String())
	}
	add(cert.Subject.CommonName)

	if len(candidates) == 0 {
		return nil, errors.New("unable to infer host from certificate")
	}
	return candidates, nil
}

// ResolveLinkHostCandidates returns host candidates inferred from the configured TLS certificate.
func ResolveLinkHostCandidates(_ string, certPathOverride string) ([]string, error) {
	certPath := strings.TrimSpace(certPathOverride)
	if certPath == "" {
		cfg, err := config.Load(config.Options{
			Path:         config.ConfigPath(layout.ServerConfigFileName),
			AllowInvalid: true,
		})
		if err != nil {
			return nil, err
		}
		certPath = strings.TrimSpace(cfg.Server.CertificateFile)
		if certPath == "" && defaultTLSConfigured() {
			certPath = defaultCertPath()
		}
	}
	if certPath == "" {
		return nil, errors.New("TLS certificate path is not configured")
	}
	return resolveLinkHostFromCertificate(certPath)
}

type trojanLinkParams struct {
	host               string
	port               int
	profile            tunnel.Profile
	tlsEnabled         bool
	pinnedPeerSHA256   string
	verifyPeerCertName string
}

func resolveTrojanLinkParams(configPath string, configDir string, hostOverride string) (trojanLinkParams, error) {
	cfg, err := config.Load(config.Options{
		Path:         configPath,
		AllowInvalid: true,
	})
	if err != nil {
		return trojanLinkParams{}, err
	}
	port := parsePortOrDefault(cfg.Server.TrojanPort, DefaultTrojanPort)
	profile, err := serverProfile(cfg.Server.Profile)
	if err != nil {
		return trojanLinkParams{}, err
	}

	certPath := strings.TrimSpace(cfg.Server.CertificateFile)
	if certPath == "" && defaultTLSConfigured() {
		certPath = defaultCertPath()
	}
	keyPath := strings.TrimSpace(cfg.Server.KeyFile)
	if keyPath == "" && defaultTLSConfigured() {
		keyPath = defaultKeyPath()
	}

	tlsEnabled := certPath != "" && keyPath != ""

	host := strings.TrimSpace(hostOverride)
	if host == "" && tlsEnabled {
		candidates, err := resolveLinkHostFromCertificate(certPath)
		if err != nil {
			return trojanLinkParams{}, err
		}
		host = candidates[0]
	}
	if host == "" {
		return trojanLinkParams{}, errors.New("host is required when TLS is disabled")
	}

	pinnedSHA := ""
	verifyName := ""
	if tlsEnabled {
		if selfSigned, err := isSelfSignedCertificatePath(certPath); err == nil && selfSigned {
			if fp, err := certificateFingerprintSHA256(certPath); err == nil {
				pinnedSHA = fp
				verifyName = host
			}
		}
	}

	return trojanLinkParams{
		host:               host,
		port:               port,
		profile:            profile,
		tlsEnabled:         tlsEnabled,
		pinnedPeerSHA256:   pinnedSHA,
		verifyPeerCertName: verifyName,
	}, nil
}
