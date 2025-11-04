package cli

import (
	"github.com/NlightN22/xray-p2p/go/internal/deploy"
	"github.com/NlightN22/xray-p2p/go/internal/version"
)

func buildDeploymentPackage(opts deployOptions) (string, error) {
	return deploy.BuildPackage(deploy.PackageOptions{
		RemoteHost: opts.remoteHost,
		Version:    version.Current(),
	})
}
