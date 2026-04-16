//go:build linux || windows

package client

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

// AddEndpoint updates the existing client installation with a new endpoint.
func AddEndpoint(ctx context.Context, opts InstallOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}
	base, err := buildClientInstallBase(installDir, "", opts)
	if err != nil {
		return err
	}
	_, err = applyClientEndpointConfig("", base.configFile, endpointConfig{
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
	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
}
