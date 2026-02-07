package server

import (
	"errors"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

// DefaultTrojanPort specifies the default inbound port for the xray-core service.
const DefaultTrojanPort = 58443

// DefaultServerConfigDir is the default directory name for server configuration files.
const DefaultServerConfigDir = layout.ServerConfigDir

// ErrUnsupported indicates that the requested operation is not supported on this platform.
var ErrUnsupported = errors.New("xp2p: server installation is not supported on this platform")

// ErrServiceUnsupported indicates that service mode is unavailable on this platform.
var ErrServiceUnsupported = errors.New("xp2p: server service is not supported on this platform")

// InstallOptions describes how the server-side components should be provisioned.
type InstallOptions struct {
	InstallDir      string
	ConfigDir       string
	Port            string
	CertificateStore string
	CertificateFile string
	KeyFile         string
	Host            string
	Force           bool
	RelaxedPathValidation bool
	TunEnabled      bool
	TunEnabledSet   bool
	TunName         string
	TunMTU          int
	TunAddr         string
}

// CertificateOptions describes how TLS material should be provisioned for an existing installation.
type CertificateOptions struct {
	InstallDir      string
	ConfigDir       string
	CertificateStore string
	CertificateFile string
	KeyFile         string
	Host            string
	Force           bool
	RelaxedPathValidation bool
	TunEnabled      bool
	TunEnabledSet   bool
	TunName         string
	TunMTU          int
	TunAddr         string
}

// RunOptions controls execution of the xray-core process.
type RunOptions struct {
	InstallDir   string
	ConfigDir    string
	ErrorLogPath string
	TunEnabled   bool
	TunName      string
	TunMTU       int
	TunAddr      string
}

// ServiceOptions controls execution of the managed server service.
type ServiceOptions struct {
	InstallDir   string
	ConfigDir    string
	XrayLogPath  string
	DiagPort     string
	MaxRestarts  int
	RestartDelay time.Duration
	TunEnabled   bool
	TunName      string
	TunMTU       int
	TunAddr      string
}

// RemoveOptions controls removal of the server-side components.
type RemoveOptions struct {
	InstallDir    string
	ConfigDir     string
	KeepFiles     bool
	IgnoreMissing bool
	TunName       string
}
