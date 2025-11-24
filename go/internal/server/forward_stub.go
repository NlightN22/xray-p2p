//go:build !windows && !linux

package server

import (
	"errors"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
)

// ForwardAddOptions describes server forward creation.
type ForwardAddOptions struct {
	InstallDir    string
	ConfigDir     string
	Target        string
	ListenAddress string
	ListenPort    int
	Protocol      forward.Protocol
	BasePort      int
}

// ForwardAddResult captures the applied forward alongside routing status.
type ForwardAddResult struct {
	Rule   forward.Rule
	Routed bool
}

// ForwardRemoveOptions controls server forward removal.
type ForwardRemoveOptions struct {
	InstallDir string
	ConfigDir  string
	Selector   forward.Selector
}

// ForwardListOptions configures forward enumeration.
type ForwardListOptions struct {
	InstallDir string
	ConfigDir  string
}

// AddForward is not supported on this platform.
func AddForward(ForwardAddOptions) (ForwardAddResult, error) {
	return ForwardAddResult{}, ErrUnsupported
}

// RemoveForward is not supported on this platform.
func RemoveForward(ForwardRemoveOptions) (forward.Rule, error) {
	return forward.Rule{}, ErrUnsupported
}

// ListForwards is not supported on this platform.
func ListForwards(ForwardListOptions) ([]forward.Rule, error) {
	return nil, ErrUnsupported
}
