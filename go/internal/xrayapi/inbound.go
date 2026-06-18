package xrayapi

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	commonnet "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonnet"
	coreconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/coreconfig"
	dokodemoconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/dokodemoconfig"
	proxymanconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/proxymanconfig"
)

var allowedForwardInboundFields = map[string]struct{}{
	"remark":   {},
	"tag":      {},
	"listen":   {},
	"port":     {},
	"protocol": {},
	"settings": {},
}

var allowedDokodemoSettingsFields = map[string]struct{}{
	"address":        {},
	"port":           {},
	"network":        {},
	"followRedirect": {},
}

func InboundFromMap(inbound map[string]any) (*coreconfig.InboundHandlerConfig, error) {
	if err := rejectUnknownKeys(inbound, allowedForwardInboundFields, "inbound"); err != nil {
		return nil, err
	}
	protocol, _ := inbound["protocol"].(string)
	if protocol != "dokodemo-door" {
		return nil, fmt.Errorf("unsupported inbound protocol %q", protocol)
	}
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
