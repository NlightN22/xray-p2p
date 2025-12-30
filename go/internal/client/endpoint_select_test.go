package client

import "testing"

func TestSelectEndpointByHostDefaultsToFirstMatch(t *testing.T) {
	endpoints := []clientEndpointRecord{
		{Hostname: "edge.example", Tag: "proxy-a"},
		{Hostname: "edge.example", Tag: "proxy-b"},
	}

	ep, idx, err := selectEndpointByHost(endpoints, "edge.example", "", 0)
	if err != nil {
		t.Fatalf("selectEndpointByHost failed: %v", err)
	}
	if idx != 0 || ep.Tag != "proxy-a" {
		t.Fatalf("unexpected selection: idx=%d tag=%s", idx, ep.Tag)
	}
}

func TestSelectEndpointByHostUsesIndex(t *testing.T) {
	endpoints := []clientEndpointRecord{
		{Hostname: "edge.example", Tag: "proxy-a"},
		{Hostname: "edge.example", Tag: "proxy-b"},
	}

	ep, idx, err := selectEndpointByHost(endpoints, "edge.example", "", 2)
	if err != nil {
		t.Fatalf("selectEndpointByHost failed: %v", err)
	}
	if idx != 1 || ep.Tag != "proxy-b" {
		t.Fatalf("unexpected selection: idx=%d tag=%s", idx, ep.Tag)
	}
}

func TestSelectEndpointByHostRejectsOutOfRangeIndex(t *testing.T) {
	endpoints := []clientEndpointRecord{
		{Hostname: "edge.example", Tag: "proxy-a"},
	}

	if _, _, err := selectEndpointByHost(endpoints, "edge.example", "", 2); err == nil {
		t.Fatal("expected error for out of range index")
	}
}

func TestSelectEndpointByHostUsesTag(t *testing.T) {
	endpoints := []clientEndpointRecord{
		{Hostname: "edge.example", Tag: "proxy-a"},
		{Hostname: "edge.example", Tag: "proxy-b"},
	}

	ep, idx, err := selectEndpointByHost(endpoints, "edge.example", "proxy-b", 0)
	if err != nil {
		t.Fatalf("selectEndpointByHost failed: %v", err)
	}
	if idx != 1 || ep.Tag != "proxy-b" {
		t.Fatalf("unexpected selection: idx=%d tag=%s", idx, ep.Tag)
	}
}

func TestSelectEndpointByHostTagIgnoresHost(t *testing.T) {
	endpoints := []clientEndpointRecord{
		{Hostname: "edge.example", Tag: "proxy-a"},
		{Hostname: "other.example", Tag: "proxy-b"},
	}

	ep, idx, err := selectEndpointByHost(endpoints, "mismatch.example", "proxy-b", 0)
	if err != nil {
		t.Fatalf("selectEndpointByHost failed: %v", err)
	}
	if idx != 1 || ep.Tag != "proxy-b" {
		t.Fatalf("unexpected selection: idx=%d tag=%s", idx, ep.Tag)
	}
}
