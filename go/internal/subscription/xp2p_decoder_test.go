package subscription

import (
	"encoding/json"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
)

func TestXP2PControlDecoderDefaultsLegacyTrojanProfile(t *testing.T) {
	data, err := json.Marshal(controlplane.Subscription{Host: "edge.example", Port: 443})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := (XP2PControlDecoder{Credential: "shared-credential", UserLabel: "client"}).Decode(RawSnapshot{Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Offers) != 1 || snapshot.Offers[0].Endpoint.Protocol != "trojan" || snapshot.Offers[0].Credential != "shared-credential" {
		t.Fatalf("unexpected legacy XP2P offer: %+v", snapshot.Offers)
	}
}

func TestXP2PControlDecoderRejectsProfileFieldMismatch(t *testing.T) {
	data, err := json.Marshal(controlplane.Subscription{
		Host: "edge.example", Port: 443, Profile: "vless-tls-vision",
		Protocol: "trojan", Transport: "tcp", Security: "tls",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = (XP2PControlDecoder{Credential: "550e8400-e29b-41d4-a716-446655440000"}).Decode(RawSnapshot{Data: data})
	if err == nil {
		t.Fatal("mismatched XP2P profile fields were accepted")
	}
}
