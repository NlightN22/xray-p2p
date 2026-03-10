//go:build !linux && !windows

package client

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
)

// Generic stubs for unsupported platforms.

// type shims to satisfy references on unsupported platforms
type (
	ListOptions struct {
		InstallDir string
		ConfigDir  string
	}
	EndpointRecord struct {
		Hostname             string
		Tag                  string
		Address              string
		Port                 int
		User                 string
		ServerName           string
		AllowInsecure        bool
		PinnedPeerCertSHA256 string
		VerifyPeerCertByName string
	}
	ReverseListOptions struct {
		InstallDir string
		ConfigDir  string
	}
	ReverseRecord struct {
		Tag         string
		Host        string
		User        string
		Domain      string
		EndpointTag string
		Bridge      bool
		DirectRule  bool
	}
	RemoveEndpointOptions struct {
		InstallDir string
		ConfigDir  string
		Target     string
	}
	ForwardAddOptions struct {
		InstallDir    string
		ConfigDir     string
		Target        string
		ListenAddress string
		ListenPort    int
		Protocol      forward.Protocol
		BasePort      int
	}
	ForwardAddResult struct {
		Rule   forward.Rule
		Routed bool
	}
	ForwardRemoveOptions struct {
		InstallDir string
		ConfigDir  string
		Selector   forward.Selector
	}
	ForwardListOptions struct {
		InstallDir string
		ConfigDir  string
	}
	RedirectAddOptions struct {
		InstallDir string
		ConfigDir  string
		CIDR       string
		Domain     string
		Tag        string
		Hostname   string
		TunEnabled bool
		TunName    string
	}
	RedirectRemoveOptions struct {
		InstallDir string
		ConfigDir  string
		CIDR       string
		Domain     string
		Tag        string
		Hostname   string
		TunEnabled bool
		TunName    string
	}
	RedirectListOptions struct {
		InstallDir string
		ConfigDir  string
	}
	RedirectRecord struct {
		Type     string
		Value    string
		CIDR     string
		Domain   string
		Tag      string
		Hostname string
	}
	ModeOptions struct {
		InstallDir string
		ConfigDir  string
		TunEnabled bool
		TunName    string
		TunMTU     int
		TunAddr    string
	}
)

func Install(_ context.Context, _ InstallOptions) error {
	return ErrUnsupported
}

func Remove(_ context.Context, _ RemoveOptions) error {
	return ErrUnsupported
}

func Run(_ context.Context, _ RunOptions) error {
	return ErrUnsupported
}

func ListEndpoints(_ ListOptions) ([]EndpointRecord, error) {
	return nil, ErrUnsupported
}

func ListReverse(_ ReverseListOptions) ([]ReverseRecord, error) {
	return nil, ErrUnsupported
}

func RemoveEndpoint(_ context.Context, _ RemoveEndpointOptions) error {
	return ErrUnsupported
}

func AddForward(_ ForwardAddOptions) (ForwardAddResult, error) {
	return ForwardAddResult{}, ErrUnsupported
}

func RemoveForward(_ ForwardRemoveOptions) (forward.Rule, error) {
	return forward.Rule{}, ErrUnsupported
}

func ListForwards(_ ForwardListOptions) ([]forward.Rule, error) {
	return nil, ErrUnsupported
}

func AddRedirect(_ RedirectAddOptions) error {
	return ErrUnsupported
}

func RemoveRedirect(_ RedirectRemoveOptions) error {
	return ErrUnsupported
}

func ListRedirects(_ RedirectListOptions) ([]RedirectRecord, error) {
	return nil, ErrUnsupported
}

func ApplyMode(_ ModeOptions) error {
	return ErrUnsupported
}
