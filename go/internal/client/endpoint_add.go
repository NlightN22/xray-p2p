//go:build linux || windows

package client

import "context"

// AddEndpoint updates the existing client installation with a new endpoint.
func AddEndpoint(ctx context.Context, opts InstallOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}
	configDir, err := ResolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	base, err := buildClientInstallBase(installDir, configDir, opts)
	if err != nil {
		return err
	}
	state, err := applyClientEndpointConfig(configDir, base.configFile, endpointConfig{
		Hostname:              base.address,
		Port:                  base.portVal,
		User:                  base.user,
		Password:              base.password,
		ServerName:            base.serverName,
		ALPN:                  base.installOpts.ALPN,
		AllowInsecure:         base.installOpts.AllowInsecure,
		PinnedPeerCertSHA256:  base.installOpts.PinnedPeerCertSHA256,
		VerifyPeerCertByName:  base.installOpts.VerifyPeerCertByName,
		AllowInsecureOverride: base.installOpts.AllowInsecureOverride,
	}, base.installOpts.Force)
	if err != nil {
		return err
	}
	return saveClientAppliedState(base.appliedStateFile, state, base.installOpts.TunEnabled, base.installOpts.TunName, base.installOpts.TunMTU, base.installOpts.TunAddr)
}
