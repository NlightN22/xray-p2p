//go:build linux

package dnsforward

import (
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

type Manager struct {
	dnsConfig   string
	statePath   string
	installDir  string
	configDir   string
	forwardRole string // "client" or "server"
}

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

func NewClientManager(installDir, configDir string) (*Manager, error) {
	return newManager("client", installDir, configDir)
}

func NewServerManager(installDir, configDir string) (*Manager, error) {
	return newManager("server", installDir, configDir)
}

func newManager(role, installDir, configDir string) (*Manager, error) {
	dnsCfg, err := detectDNSConfig()
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(installDir)
	if base == "" {
		base = layout.UnixConfigRoot
	}
	return &Manager{
		dnsConfig:   dnsCfg,
		statePath:   filepath.Join(base, "dns-forward-state.json"),
		installDir:  installDir,
		configDir:   configDir,
		forwardRole: role,
	}, nil
}
