//go:build !linux && !windows

package clientcmd

import (
	"context"
	"errors"

	"github.com/NlightN22/xray-p2p/go/internal/client"
)

var errUnsupported = errors.New("xp2p client is not supported on this platform")

// minimal option/item shims for unsupported platforms
type (
	InstallOptions          = client.InstallOptions
	RemoveOptions           = client.RemoveOptions
	RunOptions              = client.RunOptions
	DeployOptions           struct{}
	ListOptions             struct{}
	ListItem                struct{}
	StateOptions            struct{}
	ForwardAddOptions       struct{}
	ForwardRemoveOptions    struct{}
	ForwardListOptions      struct{}
	ForwardListItem         struct{}
	RedirectAddOptions      struct{}
	RedirectRemoveOptions   struct{}
	RedirectListOptions     struct{}
	RedirectListItem        struct{}
	ReverseListOptions      struct{}
	ReverseListItem         struct{}
	DnsForwardAddOptions    struct{}
	DnsForwardRemoveOptions struct{}
	DnsForwardListOptions   struct{}
	DnsForwardItem          struct{}
	NatRedirectOptions      struct{}
)

func Install(_ context.Context, _ client.InstallOptions) error {
	return errUnsupported
}

func Remove(_ context.Context, _ client.RemoveOptions) error {
	return errUnsupported
}

func Run(_ context.Context, _ client.RunOptions) error {
	return errUnsupported
}

func Deploy(_ context.Context, _ DeployOptions) error {
	return errUnsupported
}

func List(_ ListOptions) ([]ListItem, error) {
	return nil, errUnsupported
}

func State(_ StateOptions) error {
	return errUnsupported
}

func ForwardAdd(_ ForwardAddOptions) (string, error) {
	return "", errUnsupported
}

func ForwardRemove(_ ForwardRemoveOptions) (string, error) {
	return "", errUnsupported
}

func ForwardList(_ ForwardListOptions) ([]ForwardListItem, error) {
	return nil, errUnsupported
}

func RedirectAdd(_ RedirectAddOptions) error {
	return errUnsupported
}

func RedirectRemove(_ RedirectRemoveOptions) error {
	return errUnsupported
}

func RedirectList(_ RedirectListOptions) ([]RedirectListItem, error) {
	return nil, errUnsupported
}

func ReverseList(_ ReverseListOptions) ([]ReverseListItem, error) {
	return nil, errUnsupported
}

func DnsForwardAdd(_ DnsForwardAddOptions) error {
	return errUnsupported
}

func DnsForwardRemove(_ DnsForwardRemoveOptions) error {
	return errUnsupported
}

func DnsForwardList(_ DnsForwardListOptions) ([]DnsForwardItem, error) {
	return nil, errUnsupported
}

func NatRedirectApply(_ NatRedirectOptions) error {
	return errUnsupported
}
