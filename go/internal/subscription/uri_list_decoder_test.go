package subscription

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

const (
	trojanFixture = "trojan://trojan-secret@edge.example:443?security=tls&type=tcp&sni=edge.example#Trojan%20Main"
	vlessFixture  = "vless://550e8400-e29b-41d4-a716-446655440000@edge.example:8443?encryption=none&flow=xtls-rprx-vision&security=tls&type=tcp&sni=edge.example#VLESS%20Main"
)

func TestURIListDecoderPreservesProtocolCredentials(t *testing.T) {
	body := trojanFixture + "\n" + vlessFixture
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	decoder := URIListDecoder{}
	snapshot, err := decoder.Decode(RawSnapshot{Source: SourceRef{ID: "external-main"}, Data: []byte(encoded), FetchedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Offers) != 2 {
		t.Fatalf("offers = %d, want 2", len(snapshot.Offers))
	}
	if snapshot.Offers[0].Endpoint.Protocol != "trojan" || snapshot.Offers[0].Credential != "trojan-secret" {
		t.Fatalf("unexpected Trojan offer: %+v", snapshot.Offers[0])
	}
	if snapshot.Offers[1].Endpoint.Protocol != "vless" || snapshot.Offers[1].Credential != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected VLESS offer: %+v", snapshot.Offers[1])
	}
	if snapshot.Offers[0].StableID == snapshot.Offers[1].StableID {
		t.Fatal("distinct offers share a stable ID")
	}
}

func TestURIListDecoderStableIDIgnoresRemarkAndOrder(t *testing.T) {
	decoder := URIListDecoder{}
	first, err := decoder.Decode(RawSnapshot{Source: SourceRef{ID: "external-main"}, Data: []byte(trojanFixture + "\n" + vlessFixture)})
	if err != nil {
		t.Fatal(err)
	}
	changedRemark := strings.Replace(trojanFixture, "Trojan%20Main", "Renamed", 1)
	second, err := decoder.Decode(RawSnapshot{Source: SourceRef{ID: "external-main"}, Data: []byte(vlessFixture + "\n" + changedRemark)})
	if err != nil {
		t.Fatal(err)
	}
	if first.Offers[0].StableID != second.Offers[1].StableID || first.Offers[1].StableID != second.Offers[0].StableID {
		t.Fatal("stable IDs changed with remark or line order")
	}
}

func TestURIListDecoderRejectsWholeSnapshot(t *testing.T) {
	tests := []string{
		strings.Replace(trojanFixture, "#", "&unsupported=required#", 1),
		trojanFixture + "\nnot-a-link",
	}
	for _, body := range tests {
		if _, err := (URIListDecoder{}).Decode(RawSnapshot{Source: SourceRef{ID: "external-main"}, Data: []byte(body)}); err == nil {
			t.Fatalf("expected snapshot rejection for %q", body)
		}
	}
}

func TestURIListDecoderIgnoresExplicitOptionalExtension(t *testing.T) {
	body := strings.Replace(trojanFixture, "#", "&x-optional-provider-note=ignored#", 1)
	snapshot, err := (URIListDecoder{}).Decode(RawSnapshot{Source: SourceRef{ID: "external-main"}, Data: []byte(body)})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Offers) != 1 || snapshot.Offers[0].Credential != "trojan-secret" {
		t.Fatalf("optional extension changed decoded offer: %+v", snapshot.Offers)
	}
}
