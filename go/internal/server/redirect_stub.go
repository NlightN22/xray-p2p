//go:build !linux && !windows

package server

// RedirectAddOptions controls server redirect creation.
type RedirectAddOptions struct {
	InstallDir string
	ConfigDir  string
	CIDR       string
	Domain     string
	Tag        string
	Hostname   string
	TunEnabled bool
	TunName    string
}

// RedirectRemoveOptions controls server redirect deletion.
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

// RedirectListOptions controls redirect enumeration.
type RedirectListOptions struct {
	InstallDir string
	ConfigDir  string
}

type ReverseListOptions struct {
	InstallDir string
	ConfigDir  string
}

type ReverseRecord struct {
	Tag         string
	Host        string
	User        string
	Domain      string
	Portal      bool
	RoutingRule bool
}

// RedirectRecord describes a server redirect.
type RedirectRecord struct {
	Type     string
	Value    string
	CIDR     string
	Domain   string
	Tag      string
	Hostname string
}

// AddRedirect is not supported on this platform.
func AddRedirect(RedirectAddOptions) error {
	return ErrUnsupported
}

// RemoveRedirect is not supported on this platform.
func RemoveRedirect(RedirectRemoveOptions) error {
	return ErrUnsupported
}

// ListRedirects is not supported on this platform.
func ListRedirects(RedirectListOptions) ([]RedirectRecord, error) {
	return nil, ErrUnsupported
}

// ReverseList is not supported on this platform.
func ReverseList(ReverseListOptions) ([]ReverseRecord, error) {
	return nil, ErrUnsupported
}
