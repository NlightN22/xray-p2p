//go:build linux || windows

package server

import (
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

const serverRedirectRulesKey = "server_redirects"

type RedirectAddOptions struct {
	InstallDir string
	ConfigDir  string
	CIDR       string
	Domain     string
	Tag        string
	Hostname   string
	NoRoutes   bool
	TunEnabled bool
	TunName    string
}

type RedirectRemoveOptions struct {
	InstallDir string
	ConfigDir  string
	CIDR       string
	Domain     string
	Tag        string
	Hostname   string
	TunEnabled bool
	TunName    string
}

type RedirectListOptions struct {
	InstallDir string
	ConfigDir  string
	Pending    bool
}

type RedirectRecord struct {
	Type     string
	Value    string
	CIDR     string
	Domain   string
	Tag      string
	Hostname string
}

type serverRedirectStore struct {
	path      string
	doc       map[string]any
	reverse   serverReverseState
	redirects []redirect.Rule
}
