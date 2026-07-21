package client

import (
	"errors"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/naming"
)

type endpointConfig struct {
	Profile               string
	Protocol              string
	Transport             string
	Security              string
	Flow                  string
	Hostname              string
	Address               string
	Port                  int
	User                  string
	Password              string
	ServerName            string
	ALPN                  []string
	AllowInsecure         bool
	PinnedPeerCertSHA256  string
	VerifyPeerCertByName  string
	AllowInsecureOverride bool
	HeartbeatMode         string
}

func applyClientEndpointConfig(configDir, configFile string, endpoint endpointConfig, force bool) (clientInstallState, error) {
	state, err := buildClientEndpointState(configDir, configFile, endpoint, force)
	if err != nil {
		return clientInstallState{}, err
	}
	if err := state.save(configFile); err != nil {
		return clientInstallState{}, err
	}
	return state, nil
}

func buildClientEndpointState(configDir, configFile string, endpoint endpointConfig, force bool) (clientInstallState, error) {
	_ = configDir
	host := strings.TrimSpace(endpoint.Hostname)
	if host == "" {
		return clientInstallState{}, errors.New("endpoint hostname is required")
	}
	address := strings.TrimSpace(endpoint.Address)
	if address == "" {
		address = host
	}

	tag := buildProxyTag(host)
	state, err := loadClientInstallState(configFile)
	if err != nil {
		if force && errors.Is(err, ErrClientConfigParse) {
			logging.Warn(
				"invalid client config ignored due to --force",
				"path", configFile,
				"error", err,
			)
			state = clientInstallState{}
		} else {
			return clientInstallState{}, err
		}
	}

	allowValue := endpoint.AllowInsecure
	pinnedSHA256 := strings.TrimSpace(endpoint.PinnedPeerCertSHA256)
	verifyPeer := strings.TrimSpace(endpoint.VerifyPeerCertByName)
	if pinnedSHA256 != "" {
		allowValue = false
		if verifyPeer == "" {
			verifyPeer = endpoint.ServerName
		}
	}

	record := clientEndpointRecord{
		Profile:              strings.TrimSpace(endpoint.Profile),
		Protocol:             strings.TrimSpace(endpoint.Protocol),
		Transport:            strings.TrimSpace(endpoint.Transport),
		Security:             strings.TrimSpace(endpoint.Security),
		Flow:                 strings.TrimSpace(endpoint.Flow),
		Hostname:             host,
		Tag:                  tag,
		Address:              address,
		Port:                 endpoint.Port,
		User:                 endpoint.User,
		Password:             endpoint.Password,
		ServerName:           endpoint.ServerName,
		ALPN:                 normalizeALPN(endpoint.ALPN),
		AllowInsecure:        allowValue,
		PinnedPeerCertSHA256: pinnedSHA256,
		VerifyPeerCertByName: verifyPeer,
		HeartbeatMode:        heartbeat.Mode(strings.TrimSpace(endpoint.HeartbeatMode)),
	}

	if err := state.upsert(record, force); err != nil {
		return clientInstallState{}, err
	}

	if _, err := state.ensureReverseChannel(record.User, record.Hostname, record.Tag); err != nil {
		return clientInstallState{}, err
	}

	if endpoint.AllowInsecureOverride {
		state.applyAllowInsecure(record.AllowInsecure)
	}
	return state, nil
}

func clientEndpointConfigured(configFile, host string, port int) (bool, error) {
	state, err := loadClientInstallState(configFile)
	if err != nil {
		return false, err
	}
	return state.hasEndpoint(host, port), nil
}

func buildProxyTag(host string) string {
	sanitized := naming.SanitizeLabel(host)
	if sanitized == "" {
		sanitized = "endpoint"
	}
	return "proxy-" + sanitized
}

func normalizeALPN(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
