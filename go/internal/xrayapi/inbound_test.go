package xrayapi

import (
	"testing"

	dokodemoconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/dokodemoconfig"
	proxymanconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/proxymanconfig"
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
