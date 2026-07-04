package xrayapi

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	commonnet "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonnet"
	commonprotocol "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonprotocol"
	commonserial "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonserial"
	coreconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/coreconfig"
	dokodemoconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/dokodemoconfig"
	internetconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/internetconfig"
	proxymanconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/proxymanconfig"
	tlsconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/tlsconfig"
	trojanconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/trojanconfig"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

var allowedForwardInboundFields = map[string]struct{}{
	"remark":         {},
	"tag":            {},
	"listen":         {},
	"port":           {},
	"protocol":       {},
	"settings":       {},
	"streamSettings": {},
}

var allowedDokodemoSettingsFields = map[string]struct{}{
	"address":        {},
	"port":           {},
	"network":        {},
	"followRedirect": {},
}

var allowedSocksSettingsFields = map[string]struct{}{
	"udp": {},
}

var allowedInboundTLSSettingsFields = map[string]struct{}{
	"certificates": {},
}

var allowedInboundCertificateFields = map[string]struct{}{
	"certificateFile": {},
	"keyFile":         {},
}

func InboundFromMap(inbound map[string]any) (*coreconfig.InboundHandlerConfig, error) {
	if err := rejectUnknownKeys(inbound, allowedForwardInboundFields, "inbound"); err != nil {
		return nil, err
	}
	protocol, _ := inbound["protocol"].(string)
	protocol = strings.TrimSpace(protocol)
	tag, _ := inbound["tag"].(string)
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, errors.New("inbound tag is required")
	}
	listen, _ := inbound["listen"].(string)
	listenAddr, err := ipOrDomain(strings.TrimSpace(listen))
	if err != nil {
		return nil, fmt.Errorf("parse inbound listen: %w", err)
	}
	listenPort, err := uint32Port(inbound["port"], "inbound port")
	if err != nil {
		return nil, err
	}
	var result *coreconfig.InboundHandlerConfig
	switch protocol {
	case "dokodemo-door":
		result, err = dokodemoInboundFromMap(inbound, tag, listenAddr, listenPort)
	case "socks":
		result, err = socksInboundFromMap(inbound, tag, listenAddr, listenPort)
	case "trojan":
		result, err = trojanInboundFromMap(inbound, tag, listenAddr, listenPort)
	case "vless":
		result, err = vlessInboundFromMap(inbound, tag, listenAddr, listenPort)
	default:
		err = fmt.Errorf("unsupported inbound protocol %q", protocol)
	}
	if err != nil {
		return nil, err
	}
	logging.Debug("xray runtime inbound encoded", "tag", tag, "protocol", protocol, "proxy_type", result.ProxySettings.Type, "proxy_bytes", len(result.ProxySettings.Value), "receiver_type", result.ReceiverSettings.Type, "receiver_bytes", len(result.ReceiverSettings.Value))
	return result, nil
}

func dokodemoInboundFromMap(inbound map[string]any, tag string, listenAddr *commonnet.IPOrDomain, listenPort uint32) (*coreconfig.InboundHandlerConfig, error) {
	settings, ok := inbound["settings"].(map[string]any)
	if !ok {
		return nil, errors.New("dokodemo settings are required")
	}
	if err := rejectUnknownKeys(settings, allowedDokodemoSettingsFields, "dokodemo settings"); err != nil {
		return nil, err
	}
	followRedirect, _ := settings["followRedirect"].(bool)
	if followRedirect {
		return nil, errors.New("dokodemo followRedirect is not supported for runtime forward apply")
	}
	target, _ := settings["address"].(string)
	targetAddr, err := ipOrDomain(strings.TrimSpace(target))
	if err != nil {
		return nil, fmt.Errorf("parse dokodemo target: %w", err)
	}
	targetPort, err := uint32Port(settings["port"], "dokodemo target port")
	if err != nil {
		return nil, err
	}
	networks, err := dokodemoNetworks(settings["network"])
	if err != nil {
		return nil, err
	}

	receiver, err := typedMessage(&proxymanconfig.ReceiverConfig{
		PortList: &commonnet.PortList{Range: []*commonnet.PortRange{{
			From: listenPort,
			To:   listenPort,
		}}},
		Listen: listenAddr,
	})
	if err != nil {
		return nil, err
	}
	proxy, err := typedMessage(&dokodemoconfig.Config{
		Address:        targetAddr,
		Port:           targetPort,
		Networks:       networks,
		FollowRedirect: false,
	})
	if err != nil {
		return nil, err
	}
	return &coreconfig.InboundHandlerConfig{
		Tag:              tag,
		ReceiverSettings: receiver,
		ProxySettings:    proxy,
	}, nil
}

func socksInboundFromMap(inbound map[string]any, tag string, listenAddr *commonnet.IPOrDomain, listenPort uint32) (*coreconfig.InboundHandlerConfig, error) {
	settings, ok := inbound["settings"].(map[string]any)
	if !ok {
		return nil, errors.New("socks settings are required")
	}
	if err := rejectUnknownKeys(settings, allowedSocksSettingsFields, "socks settings"); err != nil {
		return nil, err
	}
	udpEnabled, _ := settings["udp"].(bool)
	receiver, err := typedMessage(&proxymanconfig.ReceiverConfig{
		PortList: &commonnet.PortList{Range: []*commonnet.PortRange{{
			From: listenPort,
			To:   listenPort,
		}}},
		Listen: listenAddr,
	})
	if err != nil {
		return nil, err
	}
	proxy := protowire.AppendVarint(protowire.AppendTag(nil, 4, protowire.VarintType), boolWire(udpEnabled))
	return &coreconfig.InboundHandlerConfig{
		Tag:              tag,
		ReceiverSettings: receiver,
		ProxySettings:    &commonserial.TypedMessage{Type: "xray.proxy.socks.ServerConfig", Value: proxy},
	}, nil
}

func trojanInboundFromMap(inbound map[string]any, tag string, listenAddr *commonnet.IPOrDomain, listenPort uint32) (*coreconfig.InboundHandlerConfig, error) {
	users, err := inboundUsers(inbound, trojanInboundAccount)
	if err != nil {
		return nil, err
	}
	receiver, err := inboundReceiver(listenAddr, listenPort, inbound)
	if err != nil {
		return nil, err
	}
	proxy, err := typedMessage(&trojanconfig.ServerConfig{Users: users})
	if err != nil {
		return nil, err
	}
	return &coreconfig.InboundHandlerConfig{Tag: tag, ReceiverSettings: receiver, ProxySettings: proxy}, nil
}

func vlessInboundFromMap(inbound map[string]any, tag string, listenAddr *commonnet.IPOrDomain, listenPort uint32) (*coreconfig.InboundHandlerConfig, error) {
	users, err := inboundUsers(inbound, vlessInboundAccount)
	if err != nil {
		return nil, err
	}
	receiver, err := inboundReceiver(listenAddr, listenPort, inbound)
	if err != nil {
		return nil, err
	}
	data := []byte{}
	for _, user := range users {
		encoded, err := proto.Marshal(user)
		if err != nil {
			return nil, fmt.Errorf("marshal vless inbound user: %w", err)
		}
		data = protowire.AppendBytes(protowire.AppendTag(data, 1, protowire.BytesType), encoded)
	}
	data = protowire.AppendString(protowire.AppendTag(data, 3, protowire.BytesType), "none")
	return &coreconfig.InboundHandlerConfig{
		Tag:              tag,
		ReceiverSettings: receiver,
		ProxySettings:    &commonserial.TypedMessage{Type: "xray.proxy.vless.inbound.Config", Value: data},
	}, nil
}

type inboundAccountFunc func(map[string]any) (*commonserial.TypedMessage, error)

func inboundUsers(inbound map[string]any, account inboundAccountFunc) ([]*commonprotocol.User, error) {
	settings, ok := inbound["settings"].(map[string]any)
	if !ok {
		return nil, errors.New("inbound settings are required")
	}
	rawClients, ok := settings["clients"].([]any)
	if !ok {
		return nil, errors.New("inbound clients are required")
	}
	users := make([]*commonprotocol.User, 0, len(rawClients))
	for _, raw := range rawClients {
		client, ok := raw.(map[string]any)
		if !ok {
			return nil, errors.New("inbound client is not an object")
		}
		accountMsg, err := account(client)
		if err != nil {
			return nil, err
		}
		users = append(users, &commonprotocol.User{Email: stringFromAny(client["email"]), Account: accountMsg})
	}
	return users, nil
}

func trojanInboundAccount(client map[string]any) (*commonserial.TypedMessage, error) {
	password := stringFromAny(client["password"])
	if password == "" {
		return nil, errors.New("trojan inbound password is required")
	}
	return typedMessage(&trojanconfig.Account{Password: password})
}

func vlessInboundAccount(client map[string]any) (*commonserial.TypedMessage, error) {
	id := stringFromAny(client["id"])
	if id == "" {
		return nil, errors.New("vless inbound id is required")
	}
	return &commonserial.TypedMessage{Type: "xray.proxy.vless.Account", Value: vlessAccount(id, stringFromAny(client["flow"]), "none")}, nil
}

func boolWire(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}

func inboundReceiver(listenAddr *commonnet.IPOrDomain, listenPort uint32, inbound map[string]any) (*commonserial.TypedMessage, error) {
	stream, err := inboundStreamSettings(inbound)
	if err != nil {
		return nil, err
	}
	return typedMessage(&proxymanconfig.ReceiverConfig{
		PortList: &commonnet.PortList{Range: []*commonnet.PortRange{{
			From: listenPort,
			To:   listenPort,
		}}},
		Listen:         listenAddr,
		StreamSettings: stream,
	})
}

func inboundStreamSettings(inbound map[string]any) (*internetconfig.StreamConfig, error) {
	stream, ok := inbound["streamSettings"].(map[string]any)
	if !ok {
		return nil, errors.New("inbound streamSettings are required")
	}
	if err := rejectUnknownKeys(stream, allowedStreamSettingsFields, "streamSettings"); err != nil {
		return nil, err
	}
	if network, _ := stream["network"].(string); strings.TrimSpace(network) != "tcp" {
		return nil, fmt.Errorf("unsupported inbound stream network %q", network)
	}
	if security, _ := stream["security"].(string); strings.TrimSpace(security) != "tls" {
		return nil, fmt.Errorf("unsupported inbound stream security %q", security)
	}
	if err := validateTCPSettings(stream["tcpSettings"]); err != nil {
		return nil, err
	}
	tlsSettings, err := tlsServerSettings(stream["tlsSettings"])
	if err != nil {
		return nil, err
	}
	tlsMsg, err := typedMessage(tlsSettings)
	if err != nil {
		return nil, err
	}
	return &internetconfig.StreamConfig{
		ProtocolName:     "tcp",
		SecurityType:     tlsMsg.Type,
		SecuritySettings: []*commonserial.TypedMessage{tlsMsg},
	}, nil
}

func tlsServerSettings(raw any) (*tlsconfig.Config, error) {
	settings, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("inbound tlsSettings are required")
	}
	if err := rejectUnknownKeys(settings, allowedInboundTLSSettingsFields, "inbound tlsSettings"); err != nil {
		return nil, err
	}
	rawCertificates, ok := settings["certificates"].([]any)
	if !ok || len(rawCertificates) != 1 {
		return nil, errors.New("inbound tlsSettings require exactly one certificate")
	}
	cert, ok := rawCertificates[0].(map[string]any)
	if !ok {
		return nil, errors.New("inbound certificate is not an object")
	}
	if err := rejectUnknownKeys(cert, allowedInboundCertificateFields, "inbound certificate"); err != nil {
		return nil, err
	}
	certPath := stringFromAny(cert["certificateFile"])
	keyPath := stringFromAny(cert["keyFile"])
	if certPath == "" || keyPath == "" {
		return nil, errors.New("inbound certificateFile and keyFile are required")
	}
	certificate, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read inbound certificate %s: %w", certPath, err)
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read inbound certificate key %s: %w", keyPath, err)
	}
	return &tlsconfig.Config{Certificate: []*tlsconfig.Certificate{{
		Certificate:     certificate,
		Key:             key,
		Usage:           tlsconfig.Certificate_ENCIPHERMENT,
		CertificatePath: certPath,
		KeyPath:         keyPath,
	}}}, nil
}

func rejectUnknownKeys(values map[string]any, allowed map[string]struct{}, label string) error {
	for key := range values {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("unsupported %s field %q", label, key)
		}
	}
	return nil
}

func ipOrDomain(value string) (*commonnet.IPOrDomain, error) {
	if value == "" {
		return nil, errors.New("address is required")
	}
	if ip := net.ParseIP(value); ip != nil {
		return &commonnet.IPOrDomain{Address: &commonnet.IPOrDomain_Ip{Ip: []byte(ip)}}, nil
	}
	return &commonnet.IPOrDomain{Address: &commonnet.IPOrDomain_Domain{Domain: value}}, nil
}

func uint32Port(value any, label string) (uint32, error) {
	switch v := value.(type) {
	case int:
		if v < 1 || v > 65535 {
			return 0, fmt.Errorf("%s is invalid", label)
		}
		return uint32(v), nil
	case float64:
		if v < 1 || v > 65535 || v != float64(int(v)) {
			return 0, fmt.Errorf("%s is invalid", label)
		}
		return uint32(v), nil
	case string:
		port, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || port < 1 || port > 65535 {
			return 0, fmt.Errorf("%s is invalid", label)
		}
		return uint32(port), nil
	default:
		return 0, fmt.Errorf("%s is required", label)
	}
}

func dokodemoNetworks(value any) ([]commonnet.Network, error) {
	raw, _ := value.(string)
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "tcp":
		return []commonnet.Network{commonnet.Network_TCP}, nil
	case "udp":
		return []commonnet.Network{commonnet.Network_UDP}, nil
	case "tcp,udp", "udp,tcp":
		return []commonnet.Network{commonnet.Network_TCP, commonnet.Network_UDP}, nil
	default:
		return nil, fmt.Errorf("unsupported dokodemo network %q", raw)
	}
}
