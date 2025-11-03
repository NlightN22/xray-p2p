//go:build windows

package server

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

var (
	errTrojanInboundMissing = errors.New("xp2p: trojan inbound not found in configuration")
	errUserIDRequired       = errors.New("xp2p: user identifier is required")
	errPasswordRequired     = errors.New("xp2p: password is required")
)

// AddUser ensures that a Trojan client with the provided identifier and password exists.
// When the client is already present with the same password the operation is a no-op.
// If the client exists with a different password it is updated in-place.
func AddUser(ctx context.Context, opts AddUserOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	userID := strings.TrimSpace(opts.UserID)
	if userID == "" {
		return errUserIDRequired
	}
	password := strings.TrimSpace(opts.Password)
	if password == "" {
		return errPasswordRequired
	}

	configDir, err := resolveUserConfigDir(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "inbounds.json")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("xp2p: read %s: %w", configPath, err)
	}

	root, err := parseInbounds(contents)
	if err != nil {
		return err
	}

	trojan, err := selectTrojanInbound(root)
	if err != nil {
		return err
	}

	settings, err := extractSettings(trojan)
	if err != nil {
		return err
	}

	clients, err := extractClients(settings)
	if err != nil {
		return err
	}

	updated := false
	for idx := range clients {
		client := clients[idx]
		if !strings.EqualFold(client.Email, userID) {
			continue
		}
		if client.Password == password {
			logging.Info("xp2p server user add skipped; client already up-to-date",
				"user_id", userID,
				"config", configPath,
			)
			return nil
		}
		clients[idx].Password = password
		updated = true
		break
	}

	if !updated {
		clients = append(clients, trojanClient{
			Email:    userID,
			Password: password,
		})
		updated = true
	}

	settings["clients"] = clientsToInterfaces(clients)
	if err := writeInbounds(configPath, root); err != nil {
		return err
	}

	logging.Info("xp2p server user added or updated",
		"user_id", userID,
		"config", configPath,
		"updated", updated,
	)
	return nil
}

// RemoveUser removes the Trojan client with the provided identifier. The operation is idempotent.
func RemoveUser(ctx context.Context, opts RemoveUserOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	userID := strings.TrimSpace(opts.UserID)
	if userID == "" {
		return errUserIDRequired
	}

	configDir, err := resolveUserConfigDir(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "inbounds.json")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: %s: %w", configPath, err)
		}
		return fmt.Errorf("xp2p: read %s: %w", configPath, err)
	}

	root, err := parseInbounds(contents)
	if err != nil {
		return err
	}

	trojan, err := selectTrojanInbound(root)
	if err != nil {
		return err
	}

	settings, err := extractSettings(trojan)
	if err != nil {
		return err
	}

	clients, err := extractClients(settings)
	if err != nil {
		return err
	}

	filtered := clients[:0]
	removed := false
	for _, client := range clients {
		if strings.EqualFold(client.Email, userID) {
			removed = true
			continue
		}
		filtered = append(filtered, client)
	}

	if !removed {
		logging.Info("xp2p server user remove skipped; client not found",
			"user_id", userID,
			"config", configPath,
		)
		return nil
	}

	settings["clients"] = clientsToInterfaces(filtered)
	if err := writeInbounds(configPath, root); err != nil {
		return err
	}

	logging.Info("xp2p server user removed",
		"user_id", userID,
		"config", configPath,
	)
	return nil
}

func resolveUserConfigDir(installDir, configDir string) (string, error) {
	cfg := strings.TrimSpace(configDir)
	if cfg != "" && filepath.IsAbs(cfg) {
		return cfg, nil
	}

	base := strings.TrimSpace(installDir)
	if base == "" {
		return "", errors.New("xp2p: install directory is required when config dir is relative")
	}

	resolvedBase, err := resolveInstallDir(base)
	if err != nil {
		return "", err
	}
	return resolveConfigDir(resolvedBase, cfg)
}

func parseInbounds(contents []byte) (map[string]any, error) {
	var root map[string]any
	if err := json.Unmarshal(contents, &root); err != nil {
		return nil, fmt.Errorf("xp2p: parse inbounds.json: %w", err)
	}
	return root, nil
}

func selectTrojanInbound(root map[string]any) (map[string]any, error) {
	rawInbounds, ok := root["inbounds"]
	if !ok {
		return nil, errors.New("xp2p: inbounds.json missing \"inbounds\" array")
	}

	inbounds, ok := rawInbounds.([]any)
	if !ok {
		return nil, errors.New("xp2p: inbounds.json has invalid \"inbounds\" array")
	}

	for _, entry := range inbounds {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if protocol, _ := m["protocol"].(string); strings.EqualFold(protocol, "trojan") {
			return m, nil
		}
	}
	return nil, errTrojanInboundMissing
}

func extractSettings(inbound map[string]any) (map[string]any, error) {
	rawSettings, ok := inbound["settings"]
	if !ok {
		return nil, errors.New("xp2p: trojan inbound missing settings")
	}
	settings, ok := rawSettings.(map[string]any)
	if !ok {
		return nil, errors.New("xp2p: trojan inbound settings invalid")
	}
	return settings, nil
}

type trojanClient struct {
	Email    string
	Password string
}

func extractClients(settings map[string]any) ([]trojanClient, error) {
	rawClients, ok := settings["clients"]
	if !ok {
		return []trojanClient{}, nil
	}
	items, ok := rawClients.([]any)
	if !ok {
		return nil, errors.New("xp2p: trojan clients must be an array")
	}

	clients := make([]trojanClient, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("xp2p: trojan client entry must be an object")
		}
		password, _ := obj["password"].(string)
		email, _ := obj["email"].(string)
		clients = append(clients, trojanClient{
			Email:    email,
			Password: password,
		})
	}
	return clients, nil
}

func clientsToInterfaces(clients []trojanClient) []any {
	result := make([]any, 0, len(clients))
	for _, client := range clients {
		entry := map[string]any{
			"password": client.Password,
		}
		if strings.TrimSpace(client.Email) != "" {
			entry["email"] = client.Email
		}
		result = append(result, entry)
	}
	return result
}

func writeInbounds(path string, root map[string]any) error {
	data, err := json.MarshalIndent(root, "", "    ")
	if err != nil {
		return fmt.Errorf("xp2p: render inbounds.json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("xp2p: write %s: %w", path, err)
	}
	return nil
}

func ListUsers(ctx context.Context, opts ListUsersOptions) ([]UserLink, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	configDir, err := resolveUserConfigDir(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return nil, err
	}

	state, err := loadTrojanState(configDir)
	if err != nil {
		return nil, err
	}

	linkHost, err := resolveLinkHost(state, strings.TrimSpace(opts.Host))
	if err != nil {
		return nil, err
	}

	links := make([]UserLink, 0, len(state.clients))
	for _, client := range state.clients {
		link, err := buildTrojanLink(linkHost, state.port, client.Password, client.Email, state.tlsEnabled, state.allowInsecure)
		if err != nil {
			return nil, err
		}
		links = append(links, UserLink{
			UserID:   client.Email,
			Password: client.Password,
			Link:     link,
		})
	}
	return links, nil
}

func GetUserLink(ctx context.Context, opts UserLinkOptions) (UserLink, error) {
	if err := ctx.Err(); err != nil {
		return UserLink{}, err
	}

	configDir, err := resolveUserConfigDir(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return UserLink{}, err
	}

	state, err := loadTrojanState(configDir)
	if err != nil {
		return UserLink{}, err
	}

	linkHost, err := resolveLinkHost(state, strings.TrimSpace(opts.Host))
	if err != nil {
		return UserLink{}, err
	}

	requestedID := strings.TrimSpace(opts.UserID)
	for _, client := range state.clients {
		if requestedID != "" && !strings.EqualFold(requestedID, strings.TrimSpace(client.Email)) {
			continue
		}
		link, err := buildTrojanLink(linkHost, state.port, client.Password, client.Email, state.tlsEnabled, state.allowInsecure)
		if err != nil {
			return UserLink{}, err
		}
		return UserLink{
			UserID:   client.Email,
			Password: client.Password,
			Link:     link,
		}, nil
	}

	if requestedID == "" {
		return UserLink{}, errors.New("xp2p: no trojan users configured")
	}
	return UserLink{}, fmt.Errorf("xp2p: user %q not found", requestedID)
}

type trojanState struct {
	configDir     string
	stream        map[string]any
	clients       []trojanClient
	port          int
	tlsEnabled    bool
	allowInsecure bool
}

func loadTrojanState(configDir string) (trojanState, error) {
	configPath := filepath.Join(configDir, "inbounds.json")
	contents, err := os.ReadFile(configPath)
	if err != nil {
		return trojanState{}, fmt.Errorf("xp2p: read %s: %w", configPath, err)
	}

	root, err := parseInbounds(contents)
	if err != nil {
		return trojanState{}, err
	}

	trojan, err := selectTrojanInbound(root)
	if err != nil {
		return trojanState{}, err
	}

	settings, err := extractSettings(trojan)
	if err != nil {
		return trojanState{}, err
	}

	clients, err := extractClients(settings)
	if err != nil {
		return trojanState{}, err
	}

	stream, err := extractStreamSettings(trojan)
	if err != nil {
		return trojanState{}, err
	}

	portValue, err := extractTrojanPort(trojan)
	if err != nil {
		return trojanState{}, err
	}

	tlsEnabled := false
	if security, _ := stream["security"].(string); strings.EqualFold(strings.TrimSpace(security), "tls") {
		tlsEnabled = true
	}

	allowInsecure := false
	if tlsEnabled {
		if tlsSettings, ok := stream["tlsSettings"].(map[string]any); ok {
			if value, ok := tlsSettings["allowInsecure"].(bool); ok {
				allowInsecure = value
			}
		}
	}

	return trojanState{
		configDir:     configDir,
		stream:        stream,
		clients:       clients,
		port:          portValue,
		tlsEnabled:    tlsEnabled,
		allowInsecure: allowInsecure,
	}, nil
}

func resolveLinkHost(state trojanState, preferred string) (string, error) {
	host := strings.TrimSpace(preferred)
	if host != "" {
		return host, nil
	}
	if !state.tlsEnabled {
		return "", errors.New("xp2p: host is required when TLS is disabled")
	}
	return inferHostFromCertificate(state.configDir, state.stream)
}

func inferHostFromCertificate(configDir string, stream map[string]any) (string, error) {
	tlsSettings, ok := stream["tlsSettings"].(map[string]any)
	if !ok {
		return "", errors.New("xp2p: tlsSettings missing in trojan stream settings")
	}

	rawCerts, ok := tlsSettings["certificates"].([]any)
	if !ok || len(rawCerts) == 0 {
		return "", errors.New("xp2p: no TLS certificates configured")
	}

	entry, ok := rawCerts[0].(map[string]any)
	if !ok {
		return "", errors.New("xp2p: tls certificate entry invalid")
	}

	rawPath, _ := entry["certificateFile"].(string)
	certPath := strings.TrimSpace(rawPath)
	if certPath == "" {
		return "", errors.New("xp2p: certificateFile missing in TLS configuration")
	}

	certPath = filepath.FromSlash(certPath)
	if !filepath.IsAbs(certPath) {
		certPath = filepath.Join(configDir, certPath)
	}

	data, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("xp2p: read certificate %s: %w", certPath, err)
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return "", fmt.Errorf("xp2p: decode certificate %s: invalid PEM data", certPath)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("xp2p: parse certificate %s: %w", certPath, err)
	}

	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0], nil
	}
	if len(cert.IPAddresses) > 0 {
		return cert.IPAddresses[0].String(), nil
	}
	if strings.TrimSpace(cert.Subject.CommonName) != "" {
		return cert.Subject.CommonName, nil
	}

	return "", errors.New("xp2p: unable to infer host from certificate")
}

func extractTrojanPort(inbound map[string]any) (int, error) {
	rawPort, ok := inbound["port"]
	if !ok {
		return 0, errors.New("xp2p: trojan inbound missing port")
	}
	switch value := rawPort.(type) {
	case float64:
		return int(value), nil
	case int:
		return value, nil
	case string:
		port, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0, fmt.Errorf("xp2p: invalid trojan port %q", value)
		}
		return port, nil
	default:
		return 0, errors.New("xp2p: trojan port has unsupported type")
	}
}

func buildTrojanLink(host string, port int, password, label string, tlsEnabled, allowInsecure bool) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", errors.New("xp2p: host is required to build trojan link")
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return "", errors.New("xp2p: password is required to build trojan link")
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
		if allowInsecure {
			query.Set("allowInsecure", "1")
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
