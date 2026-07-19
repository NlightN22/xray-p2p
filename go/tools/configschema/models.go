package main

import (
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/ha"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

type clientSection struct {
	config.ClientConfig
	client.DesiredState
	Xray xrayconfig.ClientXrayConfig `toml:"xray"`
}

type clientRoot struct {
	Logging    config.LoggingConfig    `toml:"logging"`
	Client     clientSection           `toml:"client"`
	XrayAssets config.XrayAssetsConfig `toml:"xray_assets"`
}

type serverSection struct {
	config.ServerConfig
	Users                  []tunnel.User                           `toml:"users"`
	TrojanUsers            []server.LegacyTrojanUser               `toml:"trojan_users"`
	ServerRedirects        []redirect.Rule                         `toml:"server_redirects"`
	ReverseChannels        map[string]server.DesiredReverseChannel `toml:"reverse_channels"`
	Forwards               []forward.Rule                          `toml:"forwards"`
	HAGeneration           ha.Generation                           `toml:"ha_generation"`
	HALocalPeerID          string                                  `toml:"ha_local_peer_id"`
	HAPeers                []ha.Peer                               `toml:"ha_peers"`
	HAIdentityACL          string                                  `toml:"ha_identity_acl"`
	HAProvisionedResources string                                  `toml:"ha_provisioned_resources"`
	HARedirectKeys         []string                                `toml:"ha_redirect_keys"`
	Xray                   xrayconfig.ServerXrayConfig             `toml:"xray"`
}

type serverRoot struct {
	Logging    config.LoggingConfig    `toml:"logging"`
	Server     serverSection           `toml:"server"`
	XrayAssets config.XrayAssetsConfig `toml:"xray_assets"`
}
