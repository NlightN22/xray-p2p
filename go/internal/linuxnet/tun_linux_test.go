//go:build linux

package linuxnet

import "testing"

func TestTableForNameDefaults(t *testing.T) {
	if tableForName("xp2pc") != 20090 {
		t.Fatalf("expected xp2pc table 20090")
	}
	if tableForName("xp2ps") != 20091 {
		t.Fatalf("expected xp2ps table 20091")
	}
	if tableForName("custom0") != 20090 {
		t.Fatalf("expected default table 20090")
	}
}
