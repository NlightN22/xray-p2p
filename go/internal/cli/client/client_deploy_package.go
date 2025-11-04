package clientcmd

import (
	"github.com/NlightN22/xray-p2p/go/internal/deploy"
	"github.com/NlightN22/xray-p2p/go/internal/version"
)

func buildDeploymentPackage(opts deployOptions) (string, error) {
	return deploy.BuildPackage(deploy.PackageOptions{
		RemoteHost: opts.manifest.remoteHost,
		Version:    version.Current(),
		InstallDir: opts.manifest.installDir,
		TrojanPort: opts.manifest.trojanPort,
		TrojanUser: opts.manifest.trojanUser,
		TrojanPass: opts.manifest.trojanPassword,
	})
}
