package client

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/naming"
)

type endpointConfig struct {
	Hostname              string
	Port                  int
	User                  string
	Password              string
	ServerName            string
	AllowInsecure         bool
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
		return clientInstallState{}, err
	}

	allowValue := endpoint.AllowInsecure

	record := clientEndpointRecord{
		Hostname:      host,
		Tag:           tag,
		Address:       host,
		Port:          endpoint.Port,
		User:          endpoint.User,
		Password:      endpoint.Password,
		ServerName:    endpoint.ServerName,
		AllowInsecure: allowValue,
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
	if err := writeOutboundsConfig(filepath.Join(configDir, "outbounds.json"), xrayCfg.DirectOutbound, state.Endpoints); err != nil {
		return clientInstallState{}, err
	}
	if err := updateRoutingConfig(filepath.Join(configDir, "routing.json"), xrayCfg.Routing, state.Endpoints, state.Redirects, state.Reverse); err != nil {
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
