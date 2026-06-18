package xrayapi

import (
	"errors"
	"fmt"
	"strings"

	commonprotocol "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonprotocol"
	commonserial "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonserial"
	coreconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/coreconfig"
	freedomconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/freedomconfig"
	internetconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/internetconfig"
	proxymanconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/proxymanconfig"
	tlsconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/tlsconfig"
	trojanconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/trojanconfig"
	"google.golang.org/protobuf/proto"
)

var allowedOutboundFields = map[string]struct{}{
	"protocol":       {},
	"settings":       {},
	"streamSettings": {},
	"tag":            {},
	"sendThrough":    {},
}

var allowedFreedomSettingsFields = map[string]struct{}{
	"domainStrategy": {},
}

var allowedTrojanSettingsFields = map[string]struct{}{
	"servers": {},
}

var allowedTrojanServerFields = map[string]struct{}{
	"address":  {},
	"port":     {},
	"password": {},
	"email":    {},
}

var allowedStreamSettingsFields = map[string]struct{}{
	"network":     {},
	"security":    {},
	"tlsSettings": {},
	"tcpSettings": {},
}

var allowedTLSSettingsFields = map[string]struct{}{
	"allowInsecure":           {},
	"serverName":              {},
	"alpn":                    {},
	"pinnedPeerCertSha256":    {},
	"verifyPeerCertByName":    {},
	"disableSystemRoot":       {},
	"enableSessionResumption": {},
}

func OutboundFromMap(outbound map[string]any) (*coreconfig.OutboundHandlerConfig, error) {
	if err := rejectUnknownKeys(outbound, allowedOutboundFields, "outbound"); err != nil {
		return nil, err
	}
	tag, _ := outbound["tag"].(string)
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, errors.New("outbound tag is required")
	}
	protocol, _ := outbound["protocol"].(string)
	protocol = strings.TrimSpace(protocol)

	var proxy proto.Message
	var sender *proxymanconfig.SenderConfig
	var err error
	switch protocol {
	case "freedom":
		proxy, err = freedomProxySettings(outbound)
		sender, err = senderSettings(outbound, nil, err)
	case "trojan":
		proxy, err = trojanProxySettings(outbound)
		sender, err = senderSettings(outbound, trojanStreamSettings, err)
	default:
		err = fmt.Errorf("unsupported outbound protocol %q", protocol)
	}
	if err != nil {
		return nil, err
	}
	proxyMsg, err := typedMessage(proxy)
	if err != nil {
		return nil, err
	}
	senderMsg, err := typedMessage(sender)
	if err != nil {
		return nil, err
	}
	return &coreconfig.OutboundHandlerConfig{
		Tag:            tag,
		SenderSettings: senderMsg,
		ProxySettings:  proxyMsg,
	}, nil
}

func freedomProxySettings(outbound map[string]any) (*freedomconfig.Config, error) {
	settings, ok := outbound["settings"].(map[string]any)
	if !ok {
		return nil, errors.New("freedom settings are required")
	}
	if err := rejectUnknownKeys(settings, allowedFreedomSettingsFields, "freedom settings"); err != nil {
		return nil, err
	}
	strategy, err := domainStrategy(settings["domainStrategy"])
	if err != nil {
		return nil, err
	}
	return &freedomconfig.Config{DomainStrategy: strategy}, nil
}

func boolFromAny(raw any) bool {
	value, _ := raw.(bool)
	return value
}

func stringFromAny(raw any) string {
	value, _ := raw.(string)
	return strings.TrimSpace(value)
}

func trojanProxySettings(outbound map[string]any) (*trojanconfig.ClientConfig, error) {
	settings, ok := outbound["settings"].(map[string]any)
	if !ok {
		return nil, errors.New("trojan settings are required")
	}
	if err := rejectUnknownKeys(settings, allowedTrojanSettingsFields, "trojan settings"); err != nil {
		return nil, err
	}
	servers, ok := settings["servers"].([]any)
	if !ok || len(servers) != 1 {
		return nil, errors.New("trojan settings must contain exactly one server")
	}
	server, ok := servers[0].(map[string]any)
	if !ok {
		return nil, errors.New("trojan server is not an object")
	}
	if err := rejectUnknownKeys(server, allowedTrojanServerFields, "trojan server"); err != nil {
		return nil, err
	}
	address, _ := server["address"].(string)
	addr, err := ipOrDomain(strings.TrimSpace(address))
	if err != nil {
		return nil, fmt.Errorf("parse trojan server address: %w", err)
	}
	port, err := uint32Port(server["port"], "trojan server port")
	if err != nil {
		return nil, err
	}
	password, _ := server["password"].(string)
	if strings.TrimSpace(password) == "" {
		return nil, errors.New("trojan password is required")
	}
	email, _ := server["email"].(string)
	account, err := typedMessage(&trojanconfig.Account{Password: password})
	if err != nil {
		return nil, err
	}
	return &trojanconfig.ClientConfig{
		Server: &commonprotocol.ServerEndpoint{
			Address: addr,
			Port:    port,
			User: &commonprotocol.User{
				Email:   strings.TrimSpace(email),
				Account: account,
			},
		},
	}, nil
}

type streamBuilder func(map[string]any) (*internetconfig.StreamConfig, error)

func senderSettings(outbound map[string]any, buildStream streamBuilder, previous error) (*proxymanconfig.SenderConfig, error) {
	if previous != nil {
		return nil, previous
	}
	cfg := &proxymanconfig.SenderConfig{}
	if sendThrough, _ := outbound["sendThrough"].(string); strings.TrimSpace(sendThrough) != "" {
		via, err := ipOrDomain(strings.TrimSpace(sendThrough))
		if err != nil {
			return nil, fmt.Errorf("parse sendThrough: %w", err)
		}
		cfg.Via = via
	}
	if buildStream != nil {
		stream, err := buildStream(outbound)
		if err != nil {
			return nil, err
		}
		cfg.StreamSettings = stream
	}
	return cfg, nil
}

func trojanStreamSettings(outbound map[string]any) (*internetconfig.StreamConfig, error) {
	stream, ok := outbound["streamSettings"].(map[string]any)
	if !ok {
		return nil, errors.New("trojan streamSettings are required")
	}
	if err := rejectUnknownKeys(stream, allowedStreamSettingsFields, "streamSettings"); err != nil {
		return nil, err
	}
	if network, _ := stream["network"].(string); strings.TrimSpace(network) != "tcp" {
		return nil, fmt.Errorf("unsupported trojan stream network %q", network)
	}
	if security, _ := stream["security"].(string); strings.TrimSpace(security) != "tls" {
		return nil, fmt.Errorf("unsupported trojan stream security %q", security)
	}
	if err := validateTCPSettings(stream["tcpSettings"]); err != nil {
		return nil, err
	}
	tlsSettings, err := tlsClientSettings(stream["tlsSettings"])
	if err != nil {
		return nil, err
	}
	tlsMsg, err := typedMessage(tlsSettings)
	if err != nil {
		return nil, err
	}
	return &internetconfig.StreamConfig{
		ProtocolName:     "tcp",
		SecurityType:     "tls",
		SecuritySettings: []*commonserial.TypedMessage{tlsMsg},
	}, nil
}

func tlsClientSettings(raw any) (*tlsconfig.Config, error) {
	settings, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("tlsSettings are required")
	}
	if err := rejectUnknownKeys(settings, allowedTLSSettingsFields, "tlsSettings"); err != nil {
		return nil, err
	}
	if value, _ := settings["pinnedPeerCertSha256"].(string); strings.TrimSpace(value) != "" {
		return nil, errors.New("pinnedPeerCertSha256 is not supported for runtime outbound apply")
	}
	verifyName, _ := settings["verifyPeerCertByName"].(string)
	cfg := &tlsconfig.Config{
		AllowInsecure: boolFromAny(settings["allowInsecure"]),
		ServerName:    stringFromAny(settings["serverName"]),
		NextProtocol:  stringsFromAny(settings["alpn"]),
	}
	if strings.TrimSpace(verifyName) != "" {
		cfg.VerifyPeerCertByName = []string{strings.TrimSpace(verifyName)}
	}
	return cfg, nil
}

func validateTCPSettings(raw any) error {
	if raw == nil {
		return nil
	}
	settings, ok := raw.(map[string]any)
	if !ok {
		return errors.New("tcpSettings must be an object")
	}
	header, ok := settings["header"].(map[string]any)
	if !ok || len(settings) != 1 {
		return errors.New("unsupported tcpSettings")
	}
	if headerType, _ := header["type"].(string); strings.TrimSpace(headerType) != "none" || len(header) != 1 {
		return errors.New("unsupported tcpSettings header")
	}
	return nil
}

func domainStrategy(value any) (internetconfig.DomainStrategy, error) {
	raw, _ := value.(string)
	switch strings.TrimSpace(raw) {
	case "", "AsIs":
		return internetconfig.DomainStrategy_AS_IS, nil
	case "UseIP":
		return internetconfig.DomainStrategy_USE_IP, nil
	case "UseIPv4", "UseIP4":
		return internetconfig.DomainStrategy_USE_IP4, nil
	case "UseIPv6", "UseIP6":
		return internetconfig.DomainStrategy_USE_IP6, nil
	default:
		return internetconfig.DomainStrategy_AS_IS, fmt.Errorf("unsupported freedom domainStrategy %q", raw)
	}
}
