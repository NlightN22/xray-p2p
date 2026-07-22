package subscription

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPinned3XUIManifestAndGoldenFixture(t *testing.T) {
	manifestData, err := os.ReadFile("testdata/3x-ui-v2.8.11/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		CompatibilityVersion string `json:"compatibility_version"`
		PanelTag             string `json:"panel_tag"`
		PanelCommit          string `json:"panel_commit"`
		ImageIndexDigest     string `json:"image_index_digest"`
		XrayVersion          string `json:"xray_version"`
		ProductionEvidence   struct {
			ObservedAt   string `json:"observed_at"`
			Source       string `json:"source"`
			PanelVersion string `json:"panel_version"`
			XrayVersion  string `json:"xray_version"`
		} `json:"production_evidence"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.CompatibilityVersion != AdapterURIList3XUIV2811 || manifest.PanelTag != "v2.8.11" || manifest.PanelCommit != "52fdf5d" || manifest.ImageIndexDigest == "" || manifest.XrayVersion != "v26.2.6" {
		t.Fatalf("incomplete compatibility manifest: %+v", manifest)
	}
	if manifest.ProductionEvidence.ObservedAt != "2026-07-20" || manifest.ProductionEvidence.Source == "" || manifest.ProductionEvidence.PanelVersion != manifest.PanelTag || manifest.ProductionEvidence.XrayVersion != manifest.XrayVersion {
		t.Fatalf("incomplete production version evidence: %+v", manifest.ProductionEvidence)
	}
	fixture, err := os.ReadFile("testdata/3x-ui-v2.8.11/subscription.txt")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := (URIListDecoder{}).Decode(RawSnapshot{Source: SourceRef{ID: "fixture"}, Data: fixture})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Offers) != 2 {
		t.Fatalf("offers = %d, want 2", len(snapshot.Offers))
	}
}
