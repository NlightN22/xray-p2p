package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestBuildIconSet(t *testing.T) {
	pngData := buildTestPNG(t, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	iconSet, err := BuildIconSet(pngData, pngData, pngData, 16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(iconSet.Base) == 0 || len(iconSet.Enabled) == 0 || len(iconSet.Enabling) == 0 {
		t.Fatal("expected icon bytes")
	}
	if !hasIcoHeader(iconSet.Base) {
		t.Fatal("expected ico header")
	}
}

func TestBuildIconSetRejectsEmpty(t *testing.T) {
	if _, err := BuildIconSet(nil, []byte("x"), []byte("y"), 16); err == nil {
		t.Fatal("expected error for empty base png")
	}
}

func buildTestPNG(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func hasIcoHeader(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 && data[3] == 0x00
}
