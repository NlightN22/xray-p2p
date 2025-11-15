package client

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/naming"
)

type endpointConfig struct {
	Hostname      string
	Port          int
	User          string
	Password      string
	ServerName    string
	AllowInsecure bool
}

func applyClientEndpointConfig(configDir, stateFile string, endpoint endpointConfig, force bool) error {
	host := strings.TrimSpace(endpoint.Hostname)
	if host == "" {
		return errors.New("xp2p: endpoint hostname is required")
	}

	tag := buildProxyTag(host)
	state, err := loadClientInstallState(stateFile)
	if err != nil {
		return err
	}

	record := clientEndpointRecord{
		Hostname:      host,
		Tag:           tag,
		Address:       host,
		Port:          endpoint.Port,
		User:          endpoint.User,
		Password:      endpoint.Password,
		ServerName:    endpoint.ServerName,
		AllowInsecure: endpoint.AllowInsecure,
	}

	if err := state.upsert(record, force); err != nil {
		return err
	}

	if _, err := state.ensureReverseChannel(record.User, record.Tag); err != nil {
		return err
	}

	if err := state.save(stateFile); err != nil {
		return err
	}
	if err := writeOutboundsConfig(filepath.Join(configDir, "outbounds.json"), state.Endpoints); err != nil {
		return err
	}
	if err := updateRoutingConfig(filepath.Join(configDir, "routing.json"), state.Endpoints, state.Redirects, state.Reverse); err != nil {
		return err
	}
	return nil
}

func buildProxyTag(host string) string {
	sanitized := naming.SanitizeLabel(host)
	if sanitized == "" {
		sanitized = "endpoint"
	}
	return "proxy-" + sanitized
}
