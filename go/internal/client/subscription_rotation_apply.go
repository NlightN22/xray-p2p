package client

import (
	"context"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func (r subscriptionSyncRunner) applySubscriptionCandidate(ctx context.Context, candidate clientInstallState, endpoint clientEndpointRecord, index, controlPort int, credential string, rotationPending bool) (xraylive.RuntimeApplyResult, error, error) {
	result, err := r.commitSubscriptionCandidate(ctx, candidate, index, credential, rotationPending)
	if err != nil || !rotationPending || (result != xraylive.RuntimeApplyApplied && result != xraylive.RuntimeApplyNoop) {
		return result, nil, err
	}
	return result, r.acknowledge(ctx, endpoint, controlPort, credential), nil
}

func (r subscriptionSyncRunner) commitSubscriptionCandidate(ctx context.Context, candidate clientInstallState, index int, credential string, rotationPending bool) (xraylive.RuntimeApplyResult, error) {
	var verify func(context.Context) error
	if rotationPending {
		candidateEndpoint := candidate.Endpoints[index]
		verify = func(verifyCtx context.Context) error {
			return r.verifyRotation(verifyCtx, candidateEndpoint, index, credential)
		}
	}
	commit := r.commit
	if commit == nil {
		commit = commitClientSubscriptionStateVerified
	}
	return commit(ctx, candidate, verify)
}

func (r subscriptionSyncRunner) verifyRotation(ctx context.Context, endpoint clientEndpointRecord, index int, credential string) error {
	if r.probe != nil {
		return r.probe(ctx, endpoint, index, credential)
	}
	return r.verifyRotationTunnel(ctx, endpoint, index, credential)
}

func (r subscriptionSyncRunner) acknowledge(ctx context.Context, endpoint clientEndpointRecord, port int, credential string) error {
	ack := r.ack
	if ack == nil {
		ack = acknowledgeRotation
	}
	return ack(ctx, endpoint, port, credential, r.timeout)
}

func (r subscriptionSyncRunner) verifyRotationTunnel(parent context.Context, endpoint clientEndpointRecord, index int, credential string) error {
	if r.socks == "" {
		return fmt.Errorf("SOCKS tunnel is unavailable for rotation verification")
	}
	marker, err := markerIPForIndex(index)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, r.timeout)
	defer cancel()
	return ping.Run(ctx, marker, ping.Options{Count: 1, Timeout: r.timeout, Port: DiagnosticsMarkerPort, SocksProxy: r.socks, User: endpoint.User, Credential: credential, ServerName: endpoint.ServerName, AllowInsecure: endpoint.AllowInsecure, PinnedPeerCertSHA256: endpoint.PinnedPeerCertSHA256, Silent: true})
}
