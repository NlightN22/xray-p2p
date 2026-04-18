package client

import (
	"errors"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

type clientInstallState struct {
	Endpoints []clientEndpointRecord          `json:"endpoints" toml:"endpoints"`
	Redirects []redirect.Rule                 `json:"redirects,omitempty" toml:"redirects"`
	Reverse   map[string]clientReverseChannel `json:"reverse,omitempty" toml:"reverse"`
	Forwards  []forward.Rule                  `json:"forwards,omitempty" toml:"forwards"`
}

type clientEndpointRecord struct {
	Hostname             string   `json:"hostname" toml:"hostname"`
	Tag                  string   `json:"tag" toml:"tag"`
	Address              string   `json:"address" toml:"address"`
	Port                 int      `json:"port" toml:"port"`
	User                 string   `json:"user" toml:"user"`
	Password             string   `json:"password" toml:"password"`
	ServerName           string   `json:"server_name" toml:"server_name"`
	ALPN                 []string `json:"alpn,omitempty" toml:"alpn"`
	AllowInsecure        bool     `json:"allow_insecure" toml:"allow_insecure"`
	PinnedPeerCertSHA256 string   `json:"pinned_peer_cert_sha256" toml:"pinned_peer_cert_sha256"`
	VerifyPeerCertByName string   `json:"verify_peer_cert_by_name" toml:"verify_peer_cert_by_name"`
}

type clientReverseChannel struct {
	UserID      string `json:"user_id" toml:"user_id"`
	Host        string `json:"host" toml:"host"`
	Tag         string `json:"tag" toml:"tag"`
	Domain      string `json:"domain" toml:"domain"`
	EndpointTag string `json:"endpoint_tag" toml:"endpoint_tag"`
}

var ErrClientConfigParse = errors.New("client config parse error")
