package server

import "errors"

// ErrUnsupported indicates that the requested operation is not supported on this platform.
var ErrUnsupported = errors.New("xp2p: server installation is only supported on Windows")

// InstallOptions describes how the server-side components should be provisioned.
type InstallOptions struct {
	InstallDir      string
	Port            string
	Mode            string
	CertificateFile string
	KeyFile         string
	Force           bool
	StartService    bool
}

// RemoveOptions controls removal of the server-side components.
type RemoveOptions struct {
	InstallDir    string
	KeepFiles     bool
	IgnoreMissing bool
}
