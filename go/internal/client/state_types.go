package client

import (
	"errors"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

type clientInstallState struct {
	Endpoints      []clientEndpointRecord          `json:"endpoints" toml:"endpoints"`
	EndpointGroups []endpointGroup                 `json:"endpoint_groups,omitempty" toml:"endpoint_groups"`
	Redirects      []redirect.Rule                 `json:"redirects,omitempty" toml:"redirects"`
	Reverse        map[string]clientReverseChannel `json:"reverse,omitempty" toml:"reverse"`
	Forwards       []forward.Rule                  `json:"forwards,omitempty" toml:"forwards"`
	baseDigest     string
}

type endpointGroup struct {
	GroupID            string            `json:"group_id" toml:"group_id"`
	Tag                string            `json:"tag" toml:"tag"`
	Members            []string          `json:"members" toml:"members"`
	Mode               endpointGroupMode `json:"mode,omitempty" toml:"mode"`
	FailureThreshold   int               `json:"failure_threshold,omitempty" toml:"failure_threshold"`
	SuccessThreshold   int               `json:"success_threshold,omitempty" toml:"success_threshold"`
	CooldownSeconds    int               `json:"cooldown_seconds,omitempty" toml:"cooldown_seconds"`
	MinimumHoldSeconds int               `json:"minimum_hold_seconds,omitempty" toml:"minimum_hold_seconds"`
	AutomaticFailback  bool              `json:"automatic_failback,omitempty" toml:"automatic_failback"`
	ManualActiveTag    string            `json:"manual_active_tag,omitempty" toml:"manual_active_tag"`
}

type endpointGroupMode string

const (
	endpointGroupModeAutomatic endpointGroupMode = "automatic"
	endpointGroupModeManual    endpointGroupMode = "manual"
	endpointGroupModeDisabled  endpointGroupMode = "disabled"
)

type clientEndpointRecord struct {
	Profile              string   `json:"profile,omitempty" toml:"profile,omitempty"`
	Protocol             string   `json:"protocol,omitempty" toml:"protocol,omitempty"`
	Transport            string   `json:"transport,omitempty" toml:"transport,omitempty"`
	Security             string   `json:"security,omitempty" toml:"security,omitempty"`
	Flow                 string   `json:"flow,omitempty" toml:"flow,omitempty"`
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
	Disabled             bool     `json:"disabled,omitempty" toml:"disabled,omitempty"`
}

type clientReverseChannel struct {
	ChannelID   string `json:"channel_id,omitempty" toml:"channel_id,omitempty"`
	UserID      string `json:"user_id" toml:"user_id"`
	Host        string `json:"host" toml:"host"`
	Tag         string `json:"tag" toml:"tag"`
	Domain      string `json:"domain" toml:"domain"`
	EndpointTag string `json:"endpoint_tag" toml:"endpoint_tag"`
	GroupTag    string `json:"group_tag,omitempty" toml:"group_tag,omitempty"`
	Disabled    bool   `json:"disabled,omitempty" toml:"disabled,omitempty"`
}

// Schema aliases expose the persisted Desired contract without duplicating it.
type DesiredState = clientInstallState
type DesiredEndpoint = clientEndpointRecord
type DesiredEndpointGroup = endpointGroup
type DesiredReverseChannel = clientReverseChannel

var ErrClientConfigParse = errors.New("client config parse error")
