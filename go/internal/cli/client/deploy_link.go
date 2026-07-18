package clientcmd

import (
	"strings"

	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
)

func buildDeployLink(opts *deployOptions) (string, error) {
	installDir := ""
	if opts.manifest.installDirSet {
		installDir = strings.TrimSpace(opts.manifest.installDir)
	}
	manifest := spec.Manifest{
		Host:           strings.TrimSpace(opts.runtime.serverHost),
		Version:        2,
		InstallDir:     installDir,
		Profile:        strings.TrimSpace(opts.manifest.profile),
		TrojanPort:     strings.TrimSpace(opts.manifest.trojanPort),
		TrojanUser:     strings.TrimSpace(opts.manifest.trojanUser),
		TrojanPassword: strings.TrimSpace(opts.manifest.trojanPassword),
	}
	linkURL, enc, err := deploylink.Build(opts.runtime.remoteHost, opts.runtime.deployPort, manifest, deployLinkTTL)
	if err != nil {
		return "", err
	}
	opts.runtime.ciphertext = enc.Ciphertext
	return linkURL, nil
}
