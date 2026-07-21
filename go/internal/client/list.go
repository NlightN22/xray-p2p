//go:build linux || windows

package client

import (
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
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
	TLSMode       string
	Disabled      bool
	HeartbeatMode string
	HeartbeatID   string
	Link          string
}

// ListEndpoints returns all configured endpoints.
func ListEndpoints(opts ListOptions) ([]EndpointRecord, error) {
	state, err := loadClientInstallState(config.ConfigPath(layout.ClientConfigFileName))
	if err != nil {
		return nil, err
	}
	return toEndpointRecords(state), nil
}

func toEndpointRecords(state clientInstallState) []EndpointRecord {
	records := make([]EndpointRecord, 0, len(state.Endpoints))
	for index, ep := range state.Endpoints {
		markerHost, _ := markerIPForIndex(index)
		records = append(records, EndpointRecord{
			Hostname:      ep.Hostname,
			Tag:           ep.Tag,
			Address:       ep.Address,
			Port:          ep.Port,
			User:          ep.User,
			ServerName:    ep.ServerName,
			AllowInsecure: ep.AllowInsecure,
			TLSMode:       endpointTLSMode(ep),
			Disabled:      ep.Disabled,
			HeartbeatMode: string(ep.HeartbeatMode),
			HeartbeatID:   heartbeatEndpointID(ep, markerHost, DiagnosticsMarkerPort),
			Link:          buildEndpointTrojanLink(ep),
		})
	}
	return records
}

func buildEndpointTrojanLink(ep clientEndpointRecord) string {
	address := strings.TrimSpace(ep.Address)
	password := strings.TrimSpace(ep.Password)
	if address == "" || ep.Port <= 0 || password == "" {
		return ""
	}
	endpoint, err := tunnel.DefaultProfile(tunnel.ProfileTrojanTLS)
	if err != nil {
		return ""
	}
	endpoint.Host = address
	endpoint.Port = ep.Port
	if strings.TrimSpace(ep.ServerName) == "" {
		endpoint.Security = "none"
	} else {
		endpoint.ServerName = strings.TrimSpace(ep.ServerName)
	}
	endpoint.TLS = tunnel.TLSMetadata{
		ALPN:                 normalizeALPN(ep.ALPN),
		AllowInsecure:        ep.AllowInsecure,
		PinnedPeerCertSHA256: strings.TrimSpace(ep.PinnedPeerCertSHA256),
		VerifyPeerCertByName: strings.TrimSpace(ep.VerifyPeerCertByName),
	}
	link, err := tunnel.RenderLink(tunnel.Link{
		Endpoint: endpoint,
		User:     tunnel.User{UserLabel: ep.User, Credential: password},
	})
	if err != nil {
		return ""
	}
	return link
}
