package xrayapi

import (
	"os"
	"path/filepath"
	"testing"

	commonserial "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonserial"
	dokodemoconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/dokodemoconfig"
	proxymanconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/proxymanconfig"
	trojanconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/trojanconfig"
	"google.golang.org/protobuf/proto"
)

func TestInboundFromMapConvertsForwardDokodemo(t *testing.T) {
	inbound, err := InboundFromMap(map[string]any{
		"remark":   "forward:192.0.2.10:22",
		"tag":      "in_53331",
		"listen":   "127.0.0.1",
		"port":     float64(53331),
		"protocol": "dokodemo-door",
		"settings": map[string]any{
			"address":        "192.0.2.10",
			"port":           float64(22),
			"network":        "tcp,udp",
			"followRedirect": false,
		},
	})
	if err != nil {
		t.Fatalf("InboundFromMap: %v", err)
	}
	if inbound.GetTag() != "in_53331" {
		t.Fatalf("tag = %q", inbound.GetTag())
	}
	if inbound.GetReceiverSettings().GetType() != "xray.app.proxyman.ReceiverConfig" {
		t.Fatalf("receiver type = %q", inbound.GetReceiverSettings().GetType())
	}
	receiver := &proxymanconfig.ReceiverConfig{}
	if err := proto.Unmarshal(inbound.GetReceiverSettings().GetValue(), receiver); err != nil {
		t.Fatalf("unmarshal receiver: %v", err)
	}
	if got := receiver.GetPortList().GetRange()[0].GetFrom(); got != 53331 {
		t.Fatalf("listen port = %d", got)
	}
	if inbound.GetProxySettings().GetType() != "xray.proxy.dokodemo.Config" {
		t.Fatalf("proxy type = %q", inbound.GetProxySettings().GetType())
	}
	proxy := &dokodemoconfig.Config{}
	if err := proto.Unmarshal(inbound.GetProxySettings().GetValue(), proxy); err != nil {
		t.Fatalf("unmarshal proxy: %v", err)
	}
	if proxy.GetPort() != 22 {
		t.Fatalf("target port = %d", proxy.GetPort())
	}
	if len(proxy.GetNetworks()) != 2 {
		t.Fatalf("networks = %v", proxy.GetNetworks())
	}
}

func TestInboundFromMapRejectsUnsupportedForwardFields(t *testing.T) {
	_, err := InboundFromMap(map[string]any{
		"tag":      "in_1",
		"listen":   "127.0.0.1",
		"port":     53331,
		"protocol": "dokodemo-door",
		"sniffing": map[string]any{},
		"settings": map[string]any{
			"address":        "192.0.2.10",
			"port":           22,
			"network":        "tcp",
			"followRedirect": false,
		},
	})
	if err == nil {
		t.Fatal("expected unsupported field error")
	}
}

func TestInboundFromMapRejectsFollowRedirect(t *testing.T) {
	_, err := InboundFromMap(map[string]any{
		"tag":      "dokodemo-in",
		"listen":   "127.0.0.1",
		"port":     10090,
		"protocol": "dokodemo-door",
		"settings": map[string]any{
			"address":        "127.0.0.1",
			"port":           10090,
			"network":        "tcp",
			"followRedirect": true,
		},
	})
	if err == nil {
		t.Fatal("expected followRedirect error")
	}
}

func TestInboundFromMapConvertsTrojanTLSInbound(t *testing.T) {
	certPath, keyPath := testTLSPaths(t)
	inbound, err := InboundFromMap(map[string]any{
		"tag":      "trojan-in",
		"listen":   "0.0.0.0",
		"port":     float64(443),
		"protocol": "trojan",
		"settings": map[string]any{"clients": []any{map[string]any{
			"email": "alice@example.com", "password": "secret",
		}}},
		"streamSettings": map[string]any{
			"network": "tcp", "security": "tls",
			"tcpSettings": map[string]any{"acceptProxyProtocol": false, "header": map[string]any{"type": "none"}},
			"tlsSettings": map[string]any{"certificates": []any{map[string]any{"certificateFile": certPath, "keyFile": keyPath}}},
		},
	})
	if err != nil {
		t.Fatalf("InboundFromMap: %v", err)
	}
	if inbound.GetProxySettings().GetType() != "xray.proxy.trojan.ServerConfig" {
		t.Fatalf("proxy type = %q", inbound.GetProxySettings().GetType())
	}
	proxy := &trojanconfig.ServerConfig{}
	if err := proto.Unmarshal(inbound.GetProxySettings().GetValue(), proxy); err != nil {
		t.Fatalf("unmarshal proxy: %v", err)
	}
	if len(proxy.GetUsers()) != 1 || proxy.GetUsers()[0].GetEmail() != "alice@example.com" {
		t.Fatalf("unexpected users: %+v", proxy.GetUsers())
	}
	receiver := decodeReceiver(t, inbound.GetReceiverSettings())
	if receiver.GetStreamSettings().GetSecurityType() != "xray.transport.internet.tls.Config" {
		t.Fatalf("security type = %q", receiver.GetStreamSettings().GetSecurityType())
	}
}

func TestInboundFromMapConvertsVLESSTLSInbound(t *testing.T) {
	certPath, keyPath := testTLSPaths(t)
	inbound, err := InboundFromMap(map[string]any{
		"tag":      "trojan-in",
		"listen":   "0.0.0.0",
		"port":     float64(443),
		"protocol": "vless",
		"settings": map[string]any{"decryption": "none", "clients": []any{map[string]any{
			"email": "alice@example.com", "id": "550e8400-e29b-41d4-a716-446655440000", "flow": "xtls-rprx-vision",
		}}},
		"streamSettings": map[string]any{
			"network": "tcp", "security": "tls",
			"tlsSettings": map[string]any{"certificates": []any{map[string]any{"certificateFile": certPath, "keyFile": keyPath}}},
		},
	})
	if err != nil {
		t.Fatalf("InboundFromMap: %v", err)
	}
	if inbound.GetProxySettings().GetType() != "xray.proxy.vless.inbound.Config" {
		t.Fatalf("proxy type = %q", inbound.GetProxySettings().GetType())
	}
	receiver := decodeReceiver(t, inbound.GetReceiverSettings())
	if receiver.GetStreamSettings().GetSecurityType() != "xray.transport.internet.tls.Config" {
		t.Fatalf("security type = %q", receiver.GetStreamSettings().GetSecurityType())
	}
}

func testTLSPaths(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, []byte("certificate"), 0o600); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatalf("write certificate key: %v", err)
	}
	return certPath, keyPath
}

func decodeReceiver(t *testing.T, msg *commonserial.TypedMessage) *proxymanconfig.ReceiverConfig {
	t.Helper()
	receiver := &proxymanconfig.ReceiverConfig{}
	if err := proto.Unmarshal(msg.GetValue(), receiver); err != nil {
		t.Fatalf("unmarshal receiver: %v", err)
	}
	return receiver
}
