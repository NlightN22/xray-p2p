package xrayguard

import (
	"testing"
	"time"
)

func TestDetectorDetectsFastSocketFDSpike(t *testing.T) {
	d := NewDetector(Options{
		Window:            3 * time.Second,
		MinFDDelta:        512,
		MinFDCount:        1024,
		MinSocketRatio:    0.80,
		MaxEstablishedTCP: 200,
		Action:            "kill_xray",
	})
	start := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)

	if _, ok := d.Observe(42, Sample{Timestamp: start, FDCount: 22, SocketFDCount: 16, EstablishedTCPCount: 50}); ok {
		t.Fatal("unexpected event for baseline sample")
	}
	event, ok := d.Observe(42, Sample{Timestamp: start.Add(2 * time.Second), FDCount: 4095, SocketFDCount: 4088, EstablishedTCPCount: 60})
	if !ok {
		t.Fatal("expected FD spike event")
	}
	if event.Reason != ReasonFDSpike || event.FDDelta != 4073 || event.Action != "kill_xray" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestDetectorIgnoresHighFDWithManyEstablishedTCP(t *testing.T) {
	d := NewDetector(Options{
		Window:            3 * time.Second,
		MinFDDelta:        512,
		MinFDCount:        1024,
		MinSocketRatio:    0.80,
		MaxEstablishedTCP: 200,
	})
	start := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)

	d.Observe(42, Sample{Timestamp: start, FDCount: 500, SocketFDCount: 450, EstablishedTCPCount: 450})
	if _, ok := d.Observe(42, Sample{Timestamp: start.Add(time.Second), FDCount: 1800, SocketFDCount: 1700, EstablishedTCPCount: 1200}); ok {
		t.Fatal("unexpected event for visible TCP connection growth")
	}
}
