package clientcmd

import (
	"context"
	"errors"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

var (
	clientInstallFunc          = client.Install
	clientRemoveFunc           = client.Remove
	clientRunFunc              = client.Run
	clientServiceRunFunc       = client.RunService
	clientRemoveEndpointFunc   = client.RemoveEndpoint
	clientUpdateEndpointFunc   = client.UpdateEndpointCredentials
	clientAddEndpointFunc      = client.AddEndpoint
	clientStageEndpointFunc    = client.StageEndpoint
	clientListFunc             = client.ListEndpoints
	clientReverseListFunc      = client.ListReverse
	clientReverseToggleFunc    = client.SetReverseEnabled
	clientRedirectAddFunc      = client.AddRedirect
	clientRedirectRemoveFunc   = client.RemoveRedirect
	clientRedirectListFunc     = client.ListRedirects
	clientRedirectToggleFunc   = client.SetRedirectEnabled
	performDeployHandshakeFunc = performDeployHandshake
)

// SetInstallForTesting replaces the install boundary until the returned restore
// function is called.
func SetInstallForTesting(fn func(context.Context, client.InstallOptions) error) func() {
	previous := clientInstallFunc
	clientInstallFunc = fn
	return func() { clientInstallFunc = previous }
}

// SetRemoveForTesting replaces the full removal boundary until the returned
// restore function is called.
func SetRemoveForTesting(fn func(context.Context, client.RemoveOptions) error) func() {
	previous := clientRemoveFunc
	clientRemoveFunc = fn
	return func() { clientRemoveFunc = previous }
}

// SetStageEndpointForTesting replaces the endpoint staging boundary until the
// returned restore function is called.
func SetStageEndpointForTesting(fn func(context.Context, client.InstallOptions) error) func() {
	previous := clientStageEndpointFunc
	clientStageEndpointFunc = fn
	return func() { clientStageEndpointFunc = previous }
}

// DeployHandshakeTestResult is the external deploy response used by contract tests.
type DeployHandshakeTestResult struct {
	ExitCode int
	Link     string
}

// SetDeployHandshakeForTesting replaces the network handshake boundary until the
// returned restore function is called.
func SetDeployHandshakeForTesting(
	fn func(context.Context) (DeployHandshakeTestResult, error),
) func() {
	previous := performDeployHandshakeFunc
	performDeployHandshakeFunc = func(
		ctx context.Context,
		_ deployOptions,
	) (deployResult, deployCompletionFunc, error) {
		result, err := fn(ctx)
		if err != nil {
			err = serverDeployError{msg: err.Error()}
		}
		return deployResult{ExitCode: result.ExitCode, Link: result.Link}, nil, err
	}
	return func() { performDeployHandshakeFunc = previous }
}

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
