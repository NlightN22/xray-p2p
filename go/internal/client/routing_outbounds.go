package client

import (
	"encoding/json"
	"os"
	"runtime"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func writeOutboundsConfig(path string, direct xrayconfig.DirectOutboundConfig, endpoints []clientEndpointRecord) error {
	randomTag := direct.Tag
	udpTag := ""
	if runtime.GOOS == "windows" {
		randomTag = directRandomTagWindows
		udpTag = directUDPTagWindows
	}

	managedTags := make(map[string]struct{}, len(endpoints)+2)
	for _, ep := range endpoints {
		tag := strings.TrimSpace(ep.Tag)
		if tag != "" {
			managedTags[strings.ToLower(tag)] = struct{}{}
		}
	}
	if directTag := strings.TrimSpace(randomTag); directTag != "" {
		managedTags[strings.ToLower(directTag)] = struct{}{}
	}
	if directTag := strings.TrimSpace(udpTag); directTag != "" {
		managedTags[strings.ToLower(directTag)] = struct{}{}
	}

	existing := readExistingOutbounds(path)
	for _, raw := range existing {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		tag, ok := entry["tag"].(string)
		if !ok {
			continue
		}
		trimmed := strings.ToLower(strings.TrimSpace(tag))
		if strings.HasPrefix(trimmed, "proxy-") {
			managedTags[trimmed] = struct{}{}
		}
	}
	preserved := filterUnmanagedOutbounds(existing, managedTags)
	out := struct {
		Outbounds []any `json:"outbounds"`
	}{
		Outbounds: make([]any, 0, len(preserved)+len(endpoints)+1),
	}

	out.Outbounds = append(out.Outbounds, preserved...)
	for _, ep := range endpoints {
		out.Outbounds = append(out.Outbounds, trojanOutbound(ep))
	}

	if randomTag != "" {
		sendThrough := ""
		if runtime.GOOS != "windows" {
			sendThrough = direct.SendThrough
		}
		out.Outbounds = append(out.Outbounds, freedomOutbound(randomTag, direct, sendThrough))
	}
	if udpTag != "" {
		out.Outbounds = append(out.Outbounds, freedomOutbound(udpTag, direct, direct.SendThrough))
	}
	if err := configio.WriteJSON(path, out, configio.WriteOptions{
		AuditPath:         config.AuditLogPath(),
		KeepLastKnownGood: true,
	}); err != nil {
		return err
	}
	return nil
}

func readExistingOutbounds(path string) []any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	if raw, ok := doc["outbounds"].([]any); ok {
		return raw
	}
	return nil
}

func filterUnmanagedOutbounds(existing []any, managed map[string]struct{}) []any {
	if len(existing) == 0 {
		return nil
	}
	result := make([]any, 0, len(existing))
	for _, raw := range existing {
		entry, ok := raw.(map[string]any)
		if !ok {
			result = append(result, raw)
			continue
		}
		tag, ok := entry["tag"].(string)
		if !ok {
			result = append(result, raw)
			continue
		}
		if _, exists := managed[strings.ToLower(strings.TrimSpace(tag))]; exists {
			continue
		}
		result = append(result, raw)
	}
	return result
}

func trojanOutbound(ep clientEndpointRecord) any {
	return struct {
		Protocol       string         `json:"protocol"`
		Settings       trojanSettings `json:"settings"`
		StreamSettings streamSettings `json:"streamSettings"`
		Tag            string         `json:"tag"`
	}{
		Protocol: "trojan",
		Settings: trojanSettings{
			Servers: []trojanServer{
				{
					Address:  ep.Address,
					Port:     ep.Port,
					Password: ep.Password,
					Email:    ep.User,
				},
			},
		},
		StreamSettings: streamSettings{
			Network:  "tcp",
			Security: "tls",
			TLSSettings: tlsSettings{
				AllowInsecure:        ep.AllowInsecure,
				ServerName:           ep.ServerName,
				PinnedPeerCertSHA256: ep.PinnedPeerCertSHA256,
				VerifyPeerCertByName: ep.VerifyPeerCertByName,
			},
			TCPSettings: tcpSettings{
				Header: tcpHeader{
					Type: "http",
					Request: tcpRequest{
						Version: "1.1",
						Method:  "GET",
						Path:    []string{"/"},
						Headers: map[string][]string{
							"Host": {
								"www.bing.com",
								"www.apple.com",
							},
							"User-Agent": {
								"Mozilla/5.0",
							},
							"Accept-Encoding": {
								"gzip, deflate",
							},
							"Connection": {
								"keep-alive",
							},
						},
					},
				},
			},
		},
		Tag: ep.Tag,
	}
}

type trojanSettings struct {
	Servers []trojanServer `json:"servers"`
}

type trojanServer struct {
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type streamSettings struct {
	Network     string      `json:"network"`
	Security    string      `json:"security"`
	TLSSettings tlsSettings `json:"tlsSettings"`
	TCPSettings tcpSettings `json:"tcpSettings"`
}

type tlsSettings struct {
	AllowInsecure        bool   `json:"allowInsecure,omitempty"`
	ServerName           string `json:"serverName,omitempty"`
	PinnedPeerCertSHA256 string `json:"pinnedPeerCertSha256,omitempty"`
	VerifyPeerCertByName string `json:"verifyPeerCertByName,omitempty"`
}

type tcpSettings struct {
	Header tcpHeader `json:"header"`
}

type tcpHeader struct {
	Type    string     `json:"type"`
	Request tcpRequest `json:"request"`
}

type tcpRequest struct {
	Version string              `json:"version"`
	Method  string              `json:"method"`
	Path    []string            `json:"path"`
	Headers map[string][]string `json:"headers"`
}

func freedomOutbound(tag string, direct xrayconfig.DirectOutboundConfig, sendThrough string) any {
	sendThrough = strings.TrimSpace(sendThrough)
	return struct {
		Protocol    string          `json:"protocol"`
		Settings    freedomSettings `json:"settings"`
		Tag         string          `json:"tag"`
		SendThrough string          `json:"sendThrough,omitempty"`
	}{
		Protocol: direct.Protocol,
		Settings: freedomSettings{
			DomainStrategy: direct.DomainStrategy,
		},
		Tag:         tag,
		SendThrough: sendThrough,
	}
}

type freedomSettings struct {
	DomainStrategy string `json:"domainStrategy"`
}
