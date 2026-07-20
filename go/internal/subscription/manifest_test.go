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
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.CompatibilityVersion != AdapterURIList3XUIV2811 || manifest.PanelTag != "v2.8.11" || manifest.PanelCommit != "52fdf5d" || manifest.ImageIndexDigest == "" || manifest.XrayVersion != "v26.2.6" {
		t.Fatalf("incomplete compatibility manifest: %+v", manifest)
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
