package clientcmd

import (
	"context"
	"errors"
)

var errRemoteDeploymentNotImplemented = errors.New("remote deployment pipeline not implemented yet")

func runRemoteDeployment(ctx context.Context, opts deployOptions) error {
	return errRemoteDeploymentNotImplemented
}
