//go:build linux || windows

package client

import (
	"context"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

// AddEndpoint updates the existing client installation with a new endpoint.
func AddEndpoint(ctx context.Context, opts InstallOptions) error {
	return addEndpoint(ctx, opts, commitClientRuntimeState)
}

// StageEndpoint updates Desired only. It is used by deploy flows that apply
// changes through the service layer after the deployment handshake completes.
func StageEndpoint(ctx context.Context, opts InstallOptions) error {
	return addEndpoint(ctx, opts, func(_ context.Context, state clientInstallState) error {
		return state.save(config.ConfigPath(layout.ClientConfigFileName))
	})
}

func addEndpoint(ctx context.Context, opts InstallOptions, commit func(context.Context, clientInstallState) error) error {
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
	if !base.installOpts.Force {
		exists, err := clientEndpointConfigured(base.configFile, base.address, base.portVal)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("endpoint %s:%d already exists (use --force to update)", base.address, base.portVal)
		}
	}
	resolved, err := resolveEndpointPrimaryAddress(ctx, base.address)
	if err != nil {
		return err
	}
	state, err := buildClientEndpointState("", base.configFile, endpointConfig{
		Hostname:              base.address,
		Address:               resolved,
		Port:                  base.portVal,
		User:                  base.user,
		Password:              base.password,
		ServerName:            base.serverName,
		ALPN:                  base.installOpts.ALPN,
		AllowInsecure:         base.installOpts.AllowInsecure,
		PinnedPeerCertSHA256:  base.installOpts.PinnedPeerCertSHA256,
		VerifyPeerCertByName:  base.installOpts.VerifyPeerCertByName,
		HeartbeatMode:         base.installOpts.HeartbeatMode,
		AllowInsecureOverride: base.installOpts.AllowInsecureOverride,
	}, base.installOpts.Force)
	if err != nil {
		return err
	}
	return commit(ctx, state)
}
