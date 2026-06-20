package xrayapi

import (
	"reflect"
	"testing"

	freedomconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/freedomconfig"
	internetconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/internetconfig"
	proxymanconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/proxymanconfig"
	tlsconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/tlsconfig"
	trojanconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/trojanconfig"
	"google.golang.org/protobuf/proto"
)

func TestOutboundFromMapConvertsFreedom(t *testing.T) {
	outbound, err := OutboundFromMap(map[string]any{
		"tag":      "direct",
		"protocol": "freedom",
		"settings": map[string]any{
			"domainStrategy": "UseIPv4",
		},
	})
	if err != nil {
		t.Fatalf("OutboundFromMap: %v", err)
	}
	if outbound.GetTag() != "direct" {
		t.Fatalf("tag = %q", outbound.GetTag())
	}
	if outbound.GetProxySettings().GetType() != "xray.proxy.freedom.Config" {
		t.Fatalf("proxy type = %q", outbound.GetProxySettings().GetType())
	}
	proxy := &freedomconfig.Config{}
	if err := proto.Unmarshal(outbound.GetProxySettings().GetValue(), proxy); err != nil {
		t.Fatalf("unmarshal proxy: %v", err)
	}
	if proxy.GetDomainStrategy() != internetconfig.DomainStrategy_USE_IP4 {
		t.Fatalf("domain strategy = %s", proxy.GetDomainStrategy())
	}
}

func TestOutboundFromMapConvertsTrojanTLS(t *testing.T) {
	outbound, err := OutboundFromMap(map[string]any{
		"tag":      "proxy",
		"protocol": "trojan",
		"settings": map[string]any{
			"servers": []any{
				map[string]any{
					"address":  "example.com",
					"port":     float64(443),
					"password": "secret",
					"email":    "client@example.com",
				},
			},
		},
		"streamSettings": map[string]any{
			"network":  "tcp",
			"security": "tls",
			"tcpSettings": map[string]any{
				"header": map[string]any{"type": "none"},
			},
			"tlsSettings": map[string]any{
				"serverName":           "example.com",
				"alpn":                 []any{"h2", "http/1.1"},
				"pinnedPeerCertSha256": "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
				"verifyPeerCertByName": "example.com",
			},
		},
	})
	if err != nil {
		t.Fatalf("OutboundFromMap: %v", err)
	}
	proxy := &trojanconfig.ClientConfig{}
	if err := proto.Unmarshal(outbound.GetProxySettings().GetValue(), proxy); err != nil {
		t.Fatalf("unmarshal proxy: %v", err)
	}
	if proxy.GetServer().GetPort() != 443 {
		t.Fatalf("server port = %d", proxy.GetServer().GetPort())
	}
	account := &trojanconfig.Account{}
	if err := proto.Unmarshal(proxy.GetServer().GetUser().GetAccount().GetValue(), account); err != nil {
		t.Fatalf("unmarshal account: %v", err)
	}
	if account.GetPassword() != "secret" {
		t.Fatalf("password = %q", account.GetPassword())
	}

	sender := &proxymanconfig.SenderConfig{}
	if err := proto.Unmarshal(outbound.GetSenderSettings().GetValue(), sender); err != nil {
		t.Fatalf("unmarshal sender: %v", err)
	}
	stream := sender.GetStreamSettings()
	if stream.GetProtocolName() != "tcp" || stream.GetSecurityType() != "xray.transport.internet.tls.Config" {
		t.Fatalf("stream = %s/%s", stream.GetProtocolName(), stream.GetSecurityType())
	}
	if len(stream.GetSecuritySettings()) != 1 {
		t.Fatalf("security settings count = %d", len(stream.GetSecuritySettings()))
	}
	tls := &tlsconfig.Config{}
	if err := proto.Unmarshal(stream.GetSecuritySettings()[0].GetValue(), tls); err != nil {
		t.Fatalf("unmarshal tls: %v", err)
	}
	if tls.GetServerName() != "example.com" {
		t.Fatalf("server name = %q", tls.GetServerName())
	}
	if !reflect.DeepEqual(tls.GetNextProtocol(), []string{"h2", "http/1.1"}) {
		t.Fatalf("alpn = %v", tls.GetNextProtocol())
	}
	if !reflect.DeepEqual(tls.GetVerifyPeerCertByName(), []string{"example.com"}) {
		t.Fatalf("verify names = %v", tls.GetVerifyPeerCertByName())
	}
	if got := tls.GetPinnedPeerCertSha256(); len(got) != 1 || len(got[0]) != 32 {
		t.Fatalf("pinned cert digest = %x", got)
	}
}

func TestOutboundFromMapRejectsUnsupportedRuntimeFields(t *testing.T) {
	tests := []struct {
		name     string
		outbound map[string]any
	}{
		{
			name: "tcp header",
			outbound: trojanOutboundWithStream(map[string]any{}, map[string]any{
				"header": map[string]any{"type": "http"},
			}),
		},
		{
			name: "protocol",
			outbound: map[string]any{
				"tag":      "blocked",
				"protocol": "blackhole",
				"settings": map[string]any{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := OutboundFromMap(tt.outbound); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func trojanOutboundWithStream(tlsSettings, tcpSettings map[string]any) map[string]any {
	return map[string]any{
		"tag":      "proxy",
		"protocol": "trojan",
		"settings": map[string]any{
			"servers": []any{
				map[string]any{
					"address":  "example.com",
					"port":     443,
					"password": "secret",
				},
			},
		},
		"streamSettings": map[string]any{
			"network":     "tcp",
			"security":    "tls",
			"tcpSettings": tcpSettings,
			"tlsSettings": tlsSettings,
		},
	}
}

func TestOutboundFromMapBuildsVLESS(t *testing.T) {
	outbound, err := OutboundFromMap(map[string]any{
		"tag": "proxy-vless", "protocol": "vless",
		"settings": map[string]any{"vnext": []any{map[string]any{
			"address": "edge.example", "port": 443, "users": []any{map[string]any{
				"id": "550e8400-e29b-41d4-a716-446655440000", "email": "alice", "flow": "xtls-rprx-vision", "encryption": "none",
			}},
		}}},
		"streamSettings": map[string]any{"network": "tcp", "security": "tls", "tlsSettings": map[string]any{"serverName": "edge.example"}},
	})
	if err != nil {
		t.Fatalf("OutboundFromMap: %v", err)
	}
	if outbound.GetProxySettings().GetType() != "xray.proxy.vless.outbound.Config" {
		t.Fatalf("proxy type = %q", outbound.GetProxySettings().GetType())
	}
}
