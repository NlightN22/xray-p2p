//go:build windows || linux

package server

import (
	"context"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identity"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

var IdentitySnapshotFetcher identitysync.Fetcher

func startIdentitySyncScheduler(ctx context.Context, cfg config.Config) func() {
	provider, ok := identityProviderRef(cfg)
	if !ok {
		return func() {}
	}
	schedulerCtx, cancel := context.WithCancel(ctx)
	interval, err := time.ParseDuration(strings.TrimSpace(cfg.Server.IdentityProvider.Interval))
	if err != nil || interval <= 0 {
		interval = 15 * time.Minute
	}
	service := identitysync.Service{
		Store:    identitysync.DefaultStore(),
		Fetcher:  IdentitySnapshotFetcher,
		Allocate: identity.NewManagedUserLabel,
	}
	go func() {
		runIdentitySyncOnce(schedulerCtx, service, provider)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-schedulerCtx.Done():
				return
			case <-ticker.C:
				runIdentitySyncOnce(schedulerCtx, service, provider)
			}
		}
	}()
	return cancel
}

func runIdentitySyncOnce(ctx context.Context, service identitysync.Service, provider identitysync.ProviderRef) {
	if service.Fetcher == nil {
		logging.Warn("identity sync skipped: provider fetcher is not configured")
		return
	}
	status, result, err := service.SyncAndApply(ctx, provider, func(ctx context.Context) (string, error) {
		applyResult, applyErr := ApplyIdentityRuntime(ctx)
		return string(applyResult), applyErr
	})
	if err != nil {
		logging.Warn("identity sync failed", "provider", provider.InstanceID, "err", err)
		return
	}
	logging.Info("identity sync completed", "provider", provider.InstanceID, "status", string(status.State))
	logging.Info("identity runtime apply completed", "provider", provider.InstanceID, "result", result)
}

func identityProviderRef(cfg config.Config) (identitysync.ProviderRef, bool) {
	rawKind := strings.TrimSpace(cfg.Server.IdentityProvider.Kind)
	instanceID := strings.TrimSpace(cfg.Server.IdentityProvider.InstanceID)
	if rawKind == "" && instanceID == "" {
		return identitysync.ProviderRef{}, false
	}
	return identitysync.ProviderRef{
		InstanceID: instanceID,
		Kind:       identitysync.ProviderKind(rawKind),
		Scope:      cfg.Server.IdentityProvider.GroupIDs,
	}, true
}
