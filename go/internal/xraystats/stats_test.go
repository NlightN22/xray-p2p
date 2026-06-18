package xraystats

import "testing"

func TestParseUserStats(t *testing.T) {
	raw := []byte(`{
		"stat": [
			{"name": "user>>>alice>>>traffic>>>uplink", "value": 10},
			{"name": "user>>>alice>>>traffic>>>downlink", "value": 20},
			{"name": "user>>>bob>>>traffic>>>uplink"},
			{"name": "inbound>>>api>>>traffic>>>uplink", "value": 99}
		]
	}`)
	stats, err := ParseUserStats(raw)
	if err != nil {
		t.Fatalf("ParseUserStats: %v", err)
	}
	alice := stats["alice"]
	if alice.UploadBytes != 10 || alice.DownloadBytes != 20 || alice.TotalBytes() != 30 {
		t.Fatalf("unexpected alice stats: %+v", alice)
	}
	bob := stats["bob"]
	if bob.UploadBytes != 0 || bob.DownloadBytes != 0 {
		t.Fatalf("unexpected bob stats: %+v", bob)
	}
	if _, ok := stats["api"]; ok {
		t.Fatalf("non-user stats should be ignored: %+v", stats)
	}
}

func TestFormatByteCount(t *testing.T) {
	cases := map[uint64]string{
		0:          "0 B",
		812:        "812 B",
		1024:       "1.0 KiB",
		1048576:    "1.0 MiB",
		1256277934: "1.2 GiB",
	}
	for input, want := range cases {
		if got := FormatByteCount(input); got != want {
			t.Fatalf("FormatByteCount(%d) = %q, want %q", input, got, want)
		}
	}
}
