//go:build linux || windows

package client

import (
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

// ListOptions controls endpoint listing.
type ListOptions struct {
	InstallDir string
	ConfigDir  string
	Pending    bool
}

// EndpointRecord represents a configured client endpoint.
type EndpointRecord struct {
	Hostname      string
	Tag           string
	Address       string
	Port          int
	User          string
	ServerName    string
	AllowInsecure bool
}

// ListEndpoints returns all configured endpoints.
func ListEndpoints(opts ListOptions) ([]EndpointRecord, error) {
	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	if opts.Pending {
		statePath = filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName))
	}
	state, err := loadClientInstallState(statePath)
	if err != nil {
		return nil, err
	}

	records := make([]EndpointRecord, 0, len(state.Endpoints))
	for _, ep := range state.Endpoints {
		records = append(records, EndpointRecord{
			Hostname:      ep.Hostname,
			Tag:           ep.Tag,
			Address:       ep.Address,
			Port:          ep.Port,
			User:          ep.User,
			ServerName:    ep.ServerName,
			AllowInsecure: ep.AllowInsecure,
		})
	}
	return records, nil
}
