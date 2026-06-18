package xraystats

import (
	"context"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/xrayapi"
)

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

func TestQueryUserStatsUsesDirectXrayAPI(t *testing.T) {
	orig := queryStats
	t.Cleanup(func() { queryStats = orig })
	queryStats = func(ctx context.Context, opts xrayapi.StatsQueryOptions) ([]xrayapi.Stat, error) {
		if opts.Address != "127.0.0.1:52180" {
			t.Fatalf("address = %q, want 127.0.0.1:52180", opts.Address)
		}
		if opts.Pattern != "user>>>" {
			t.Fatalf("pattern = %q, want user>>>", opts.Pattern)
		}
		if opts.Timeout != time.Second {
			t.Fatalf("timeout = %s, want 1s", opts.Timeout)
		}
		return []xrayapi.Stat{
			{Name: "user>>>alice>>>traffic>>>uplink", Value: 10},
			{Name: "user>>>alice>>>traffic>>>downlink", Value: 20},
		}, nil
	}

	stats, err := QueryUserStats(context.Background(), QueryOptions{
		APIAddress: " 127.0.0.1:52180 ",
		Timeout:    time.Second,
	})
	if err != nil {
		t.Fatalf("QueryUserStats: %v", err)
	}
	alice := stats["alice"]
	if alice.UploadBytes != 10 || alice.DownloadBytes != 20 {
		t.Fatalf("unexpected alice stats: %+v", alice)
	}
}

func TestUserStatsFromXrayStatsRejectsNegativeCounters(t *testing.T) {
	_, err := UserStatsFromXrayStats([]xrayapi.Stat{
		{Name: "user>>>alice>>>traffic>>>uplink", Value: -1},
	})
	if err == nil {
		t.Fatal("expected error")
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
