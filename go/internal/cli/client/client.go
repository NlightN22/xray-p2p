package clientcmd

import (
	"context"
	"errors"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

var (
	clientInstallFunc        = client.Install
	clientRemoveFunc         = client.Remove
	clientRunFunc            = client.Run
	clientRemoveEndpointFunc = client.RemoveEndpoint
	clientListFunc           = client.ListEndpoints
	clientReverseListFunc    = client.ListReverse
	clientRedirectAddFunc    = client.AddRedirect
	clientRedirectRemoveFunc = client.RemoveRedirect
	clientRedirectListFunc   = client.ListRedirects
)

// Execute runs the xp2p client command tree with the provided arguments.
func Execute(ctx context.Context, cfg config.Config, args []string) int {
	cmd := NewCommand(func() config.Config { return cfg })
	cmd.SetContext(ctx)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		var exitErr interface {
			ExitCode() int
		}
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}
