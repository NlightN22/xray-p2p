package client

import (
	"errors"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

// DefaultClientConfigDir is the default directory name for client configuration files.
const DefaultClientConfigDir = layout.ClientConfigDir

// ErrUnsupported indicates that the requested operation is not supported on this platform.
var ErrUnsupported = errors.New("xp2p: client installation is not supported on this platform")

// ErrServiceUnsupported indicates that service mode is unavailable on this platform.
var ErrServiceUnsupported = errors.New("xp2p: client service is not supported on this platform")

// InstallOptions describes how the client-side components should be provisioned.
type InstallOptions struct {
	InstallDir            string
	ConfigDir             string
	ServerAddress         string
	ServerPort            string
	User                  string
	Password              string
	ServerName            string
	AllowInsecure         bool
	AllowInsecureOverride bool
	Force                 bool
}

// RunOptions controls execution of the xray-core client process.
type RunOptions struct {
	InstallDir   string
	ConfigDir    string
	ErrorLogPath string
	Heartbeat    HeartbeatOptions
}

// ServiceOptions controls execution of the managed client service.
type ServiceOptions struct {
	InstallDir   string
	ConfigDir    string
	XrayLogPath  string
	Heartbeat    HeartbeatOptions
	DiagPort     string
	MaxRestarts  int
	RestartDelay time.Duration
}

// HeartbeatOptions controls background telemetry probes.
type HeartbeatOptions struct {
	Enabled      bool
	Interval     time.Duration
	Timeout      time.Duration
	Port         string
	SocksAddress string
}

// RemoveOptions controls removal of the client-side components.
type RemoveOptions struct {
	InstallDir    string
	ConfigDir     string
	KeepFiles     bool
	IgnoreMissing bool
}
