package server

import (
	"errors"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

// DefaultTrojanPort specifies the default inbound port for the xray-core service.
const DefaultTrojanPort = 58443

// DefaultServerConfigDir is the default directory name for server configuration files.
const DefaultServerConfigDir = layout.ServerConfigDir

// ErrUnsupported indicates that the requested operation is not supported on this platform.
var ErrUnsupported = errors.New("xp2p: server installation is only supported on Windows")

// InstallOptions describes how the server-side components should be provisioned.
type InstallOptions struct {
	InstallDir      string
	ConfigDir       string
	Port            string
	CertificateFile string
	KeyFile         string
	Host            string
	Force           bool
}

// CertificateOptions describes how TLS material should be provisioned for an existing installation.
type CertificateOptions struct {
	InstallDir      string
	ConfigDir       string
	CertificateFile string
	KeyFile         string
	Host            string
	Force           bool
}

// RunOptions controls execution of the xray-core process.
type RunOptions struct {
	InstallDir   string
	ConfigDir    string
	ErrorLogPath string
}

// RemoveOptions controls removal of the server-side components.
type RemoveOptions struct {
	InstallDir    string
	KeepFiles     bool
	IgnoreMissing bool
}
