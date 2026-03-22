package client

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/naming"
)

type endpointConfig struct {
	Hostname              string
	Port                  int
	User                  string
	Password              string
	ServerName            string
	ALPN                  []string
	AllowInsecure         bool
	PinnedPeerCertSHA256  string
	VerifyPeerCertByName  string
	AllowInsecureOverride bool
}

func applyClientEndpointConfig(configDir, configFile string, endpoint endpointConfig, force bool) (clientInstallState, error) {
	host := strings.TrimSpace(endpoint.Hostname)
	if host == "" {
		return clientInstallState{}, errors.New("xp2p: endpoint hostname is required")
	}

	tag := buildProxyTag(host)
	state, err := loadClientInstallState(configFile)
	if err != nil {
		if force && errors.Is(err, ErrClientConfigParse) {
			logging.Warn(
				"xp2p: invalid client config ignored due to --force",
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
		Hostname:             host,
		Tag:                  tag,
		Address:              host,
		Port:                 endpoint.Port,
		User:                 endpoint.User,
		Password:             endpoint.Password,
		ServerName:           endpoint.ServerName,
		ALPN:                 normalizeALPN(endpoint.ALPN),
		AllowInsecure:        allowValue,
		PinnedPeerCertSHA256: pinnedSHA256,
		VerifyPeerCertByName: verifyPeer,
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

	if err := state.save(configFile); err != nil {
		return clientInstallState{}, err
	}
	xrayCfg, err := ensureClientXrayConfig(configFile)
	if err != nil {
		return clientInstallState{}, err
	}
	endpointIPs, err := resolveEndpointIPMapWithCache(context.Background(), state.Endpoints)
	if err != nil {
		return clientInstallState{}, err
	}
	if err := writeOutboundsConfig(filepath.Join(configDir, "outbounds.json"), xrayCfg.DirectOutbound, state.Endpoints, endpointIPs, true); err != nil {
		return clientInstallState{}, err
	}
	fullEnabled, fullTag, err := loadFullTunnelRouteSettings(configFile)
	if err != nil {
		return clientInstallState{}, err
	}
	routeEndpointIPs := map[string]fullTunnelEndpointIPs(nil)
	if fullEnabled {
		routeEndpointIPs, err = loadFullTunnelEndpointCache()
		if err != nil {
			return clientInstallState{}, err
		}
	}
	if err := updateRoutingConfig(filepath.Join(configDir, "routing.json"), xrayCfg.Routing, state.Endpoints, state.Redirects, state.Reverse, fullEnabled, fullTag, routeEndpointIPs, false); err != nil {
		return clientInstallState{}, err
	}
	return state, nil
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
