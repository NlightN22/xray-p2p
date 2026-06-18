//go:build !linux

package xrayguard

import (
	"context"
	"errors"
)

var ErrUnsupported = errors.New("xrayguard collector is not supported on this platform")

type unsupportedCollector struct{}

func DefaultCollector() Collector {
	return unsupportedCollector{}
}

func (unsupportedCollector) Sample(context.Context, int) (Sample, error) {
	return Sample{}, ErrUnsupported
}
