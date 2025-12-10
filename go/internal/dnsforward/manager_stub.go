//go:build !linux

package dnsforward

import (
	"context"
	"errors"
)

type Manager struct{}

type AddOptions struct {
	Domain      string
	Target      string
	WithForward bool
	Intercept   bool
	Quiet       bool
}

type RemoveOptions struct {
	Domain      string
	All         bool
	WithForward bool
	Intercept   bool
	Quiet       bool
}

type ListEntry struct {
	Domain string
	Server string
	Target string
	Labels []string
}

func NewManager(_, _ string) (*Manager, error) {
	return nil, errors.New("xp2p dns-forward is supported on OpenWrt (Linux) only")
}

func (m *Manager) Add(_ context.Context, _ AddOptions) (ListEntry, error) {
	return ListEntry{}, errors.New("xp2p dns-forward is supported on OpenWrt (Linux) only")
}

func (m *Manager) Remove(_ RemoveOptions) ([]string, error) {
	return nil, errors.New("xp2p dns-forward is supported on OpenWrt (Linux) only")
}

func (m *Manager) List() ([]ListEntry, bool, error) {
	return nil, false, errors.New("xp2p dns-forward is supported on OpenWrt (Linux) only")
}

func (m *Manager) Diagnostics(_ bool) map[string]string {
	return map[string]string{}
}
