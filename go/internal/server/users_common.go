package server

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

const serverTrojanUsersKey = "trojan_users"

type trojanClient struct {
	Email    string `json:"email" toml:"email"`
	Password string `json:"password" toml:"password"`
	Disabled bool   `json:"disabled,omitempty" toml:"disabled,omitempty"`
}

func decodeServerTrojanUsers(doc map[string]any) ([]trojanClient, error) {
	raw := doc[serverTrojanUsersKey]
	if raw == nil {
		return []trojanClient{}, nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode server trojan users: %w", err)
	}
	var users []trojanClient
	if err := json.Unmarshal(buf, &users); err != nil {
		return nil, fmt.Errorf("decode server trojan users: %w", err)
	}
	if users == nil {
		users = []trojanClient{}
	}
	return users, nil
}

func saveServerTrojanUsers(configPath string, users []trojanClient) error {
	doc, err := loadServerStateDoc(configPath)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		doc[serverTrojanUsersKey] = nil
	} else {
		doc[serverTrojanUsersKey] = users
	}
	return writeServerStateDoc(configPath, doc)
}

func clientsToInterfaces(clients []trojanClient) []any {
	result := make([]any, 0, len(clients))
	for _, client := range clients {
		if client.Disabled {
			continue
		}
		entry := map[string]any{
			"password": strings.TrimSpace(client.Password),
		}
		if strings.TrimSpace(client.Email) != "" {
			entry["email"] = strings.TrimSpace(client.Email)
		}
		result = append(result, entry)
	}
	return result
}

func buildTrojanLink(host string, port int, password, label string, tlsEnabled bool, pinnedPeerCertSHA256, verifyPeerCertByName string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", errors.New("host is required to build trojan link")
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return "", errors.New("password is required to build trojan link")
	}

	u := &url.URL{
		Scheme: "trojan",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		User:   url.User(password),
	}

	query := url.Values{}
	if tlsEnabled {
		query.Set("security", "tls")
		query.Set("sni", host)
		if strings.TrimSpace(pinnedPeerCertSHA256) != "" {
			query.Set("pinnedPeerCertSha256", strings.TrimSpace(pinnedPeerCertSHA256))
		}
		if strings.TrimSpace(verifyPeerCertByName) != "" {
			query.Set("verifyPeerCertByName", strings.TrimSpace(verifyPeerCertByName))
		}
	} else {
		query.Set("security", "none")
	}
	u.RawQuery = query.Encode()

	if trimmed := strings.TrimSpace(label); trimmed != "" {
		u.Fragment = url.QueryEscape(trimmed)
	}

	return u.String(), nil
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
		tlsEnabled:         tlsEnabled,
		pinnedPeerSHA256:   pinnedSHA,
		verifyPeerCertName: verifyName,
	}, nil
}
