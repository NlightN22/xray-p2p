package server

import "testing"

func TestResourceGrowthDetectorWarnsAfterThreeIncreasingSamples(t *testing.T) {
	var detector resourceGrowthDetector
	if detector.Observe(1, 10) || detector.Observe(2, 11) {
		t.Fatal("growth warning fired before three samples")
	}
	if !detector.Observe(3, 12) {
		t.Fatal("growth warning did not fire after three samples")
	}
	if detector.Observe(3, 12) {
		t.Fatal("stable resources did not reset growth warning")
	}
}

func TestResourceGrowthDetectorTracksFDWithoutConnections(t *testing.T) {
	var detector resourceGrowthDetector
	detector.Observe(0, 10)
	detector.Observe(0, 11)
	if !detector.Observe(0, 12) {
		t.Fatal("fd-only growth was not detected")
	}
}
