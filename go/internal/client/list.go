//go:build linux || windows

package client

import (
	"net"
	"net/url"
	"strconv"
	"strings"

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
	TLSMode       string
	Disabled      bool
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
	for _, ep := range state.Endpoints {
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

	u := &url.URL{
		Scheme: "trojan",
		Host:   net.JoinHostPort(address, strconv.Itoa(ep.Port)),
		User:   url.User(password),
	}

	query := url.Values{}
	if strings.TrimSpace(ep.ServerName) == "" {
		query.Set("security", "none")
	} else {
		query.Set("security", "tls")
		query.Set("sni", strings.TrimSpace(ep.ServerName))
	}
	if ep.AllowInsecure {
		query.Set("allowInsecure", "1")
	}
	if strings.TrimSpace(ep.PinnedPeerCertSHA256) != "" {
		query.Set("pinnedPeerCertSha256", strings.TrimSpace(ep.PinnedPeerCertSHA256))
	}
	if strings.TrimSpace(ep.VerifyPeerCertByName) != "" {
		query.Set("verifyPeerCertByName", strings.TrimSpace(ep.VerifyPeerCertByName))
	}
	if len(ep.ALPN) > 0 {
		query.Set("alpn", strings.Join(ep.ALPN, ","))
	}
	u.RawQuery = query.Encode()

	if user := strings.TrimSpace(ep.User); user != "" {
		u.Fragment = url.QueryEscape(user)
	}
	return u.String()
}
