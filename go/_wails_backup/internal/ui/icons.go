package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	ico "github.com/Kodeworks/golang-image-ico"
	"golang.org/x/image/draw"
)

type IconSet struct {
	Base     []byte
	Enabling []byte
	Enabled  []byte
}

func BuildIconSet(basePNG, enablingPNG, enabledPNG []byte, size int) (IconSet, error) {
	if size <= 0 {
		size = 64
	}
	base, err := pngToIco(basePNG, size)
	if err != nil {
		return IconSet{}, fmt.Errorf("icon base: %w", err)
	}
	enabling, err := pngToIco(enablingPNG, size)
	if err != nil {
		return IconSet{}, fmt.Errorf("icon enabling: %w", err)
	}
	enabled, err := pngToIco(enabledPNG, size)
	if err != nil {
		return IconSet{}, fmt.Errorf("icon enabled: %w", err)
	}
	return IconSet{
		Base:     base,
		Enabling: enabling,
		Enabled:  enabled,
	}, nil
}

func pngToIco(src []byte, size int) ([]byte, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("empty png data")
	}
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	resized := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.ApproxBiLinear.Scale(resized, resized.Bounds(), img, img.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := ico.Encode(&buf, resized); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
