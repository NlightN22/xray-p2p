//go:build linux || windows

package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
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
	configDir, err := ResolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	pendingConfigDir, err := config.PendingConfigDir(configDir)
	if err != nil {
		return err
	}
	base, err := buildClientInstallBase(installDir, configDir, opts)
	if err != nil {
		return err
	}
	liveConfigFile := filepath.Clean(config.LiveConfigPath(layout.ClientConfigFileName))
	if err := seedPendingClientConfig(base.configFile, liveConfigFile); err != nil {
		return err
	}
	_, err = applyClientEndpointConfig(pendingConfigDir, base.configFile, endpointConfig{
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

func seedPendingClientConfig(pendingPath, livePath string) error {
	if strings.TrimSpace(pendingPath) == "" {
		return nil
	}
	if _, err := os.Stat(pendingPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: stat client config %s: %w", pendingPath, err)
	}

	if strings.TrimSpace(livePath) == "" {
		return nil
	}
	data, err := os.ReadFile(livePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("xp2p: read client config %s: %w", livePath, err)
	}
	return configio.WriteBytes(pendingPath, data, configio.WriteOptions{
		AuditPath:         config.AuditLogPath(),
		KeepLastKnownGood: true,
		IgnoreAuditErrors: true,
	})
}
