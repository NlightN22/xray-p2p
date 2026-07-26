package client

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func TestSubscriptionCandidateUpdatesEndpointWithoutLosingCredential(t *testing.T) {
	current := clientInstallState{
		Endpoints: []clientEndpointRecord{{
			Hostname: "old.example",
			Address:  "old.example",
			Tag:      "proxy-old",
			Port:     8443,
			User:     "alice",
		}},
	}
	sub := controlplane.Subscription{
		Generation: "next",
		Protocol:   "trojan",
		Host:       "new.example",
		Port:       9443,
		ServerName: "new.example",
		TLS: controlplane.TLSMetadata{
			PinnedPeerCertSHA256: "abc123",
			VerifyPeerCertByName: "new.example",
		},
	}
	candidate, err := subscriptionCandidate(current, current.Endpoints[0], sub, "secret")
	if err != nil {
		t.Fatalf("subscriptionCandidate: %v", err)
	}
	got := candidate.Endpoints[0]
	if got.Hostname != "new.example" || got.Port != 9443 {
		t.Fatalf("endpoint not updated: %+v", got)
	}
	if got.Password != "secret" {
		t.Fatalf("credential lost: %+v", got)
	}
	if got.AllowInsecure {
		t.Fatalf("pinned endpoint must not allow insecure TLS")
	}
}

func TestRotationProbeFailurePreventsPersistenceAndAck(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	statePath := writeEndpointUpdateState(t)
	liveDir := writeClientLive(t, "previous-live")
	beforeDesired := readFile(t, statePath)
	beforeLive := readFile(t, filepath.Join(liveDir, layout.XrayConfigFileName))
	candidate, err := loadClientInstallState(config.ConfigPath(layout.ClientConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	candidate.Endpoints[0].Password = "new-credential"
	probeErr := errors.New("injected tunnel probe failure")
	acknowledged := false
	runner := subscriptionSyncRunner{
		probe: func(context.Context, clientEndpointRecord, int, string) error { return probeErr },
		ack: func(context.Context, ownedhttp.Doer, clientEndpointRecord, int, string) error {
			acknowledged = true
			return nil
		},
	}
	stubRuntimeFlow(t, true, func(ctx context.Context, opts xraylive.Options, _ xraylive.Artifacts) (xraylive.RuntimeApplyResult, error) {
		return xraylive.RuntimeApplyFailed, opts.VerifyRuntime(ctx)
	})
	if _, _, err := runner.applySubscriptionCandidate(context.Background(), candidate, candidate.Endpoints[0], 0, 62022, "new-credential", true); !errors.Is(err, probeErr) {
		t.Fatalf("probe error = %v, want %v", err, probeErr)
	}
	if acknowledged {
		t.Fatal("failed probe acknowledged rotation")
	}
	if got := readFile(t, statePath); string(got) != string(beforeDesired) {
		t.Fatalf("failed probe changed Desired:\n%s", got)
	}
	if got := readFile(t, filepath.Join(liveDir, layout.XrayConfigFileName)); string(got) != string(beforeLive) {
		t.Fatalf("failed probe changed Live: %s", got)
	}
}

func TestRotationPersistenceFailurePreventsAck(t *testing.T) {
	persistErr := errors.New("injected Desired/Live persistence failure")
	desired := "previous-desired"
	live := "previous-live"
	acknowledged := false
	runner := subscriptionSyncRunner{
		probe: func(context.Context, clientEndpointRecord, int, string) error { return nil },
		commit: func(ctx context.Context, _ clientInstallState, verify func(context.Context) error) (xraylive.RuntimeApplyResult, error) {
			if err := verify(ctx); err != nil {
				return xraylive.RuntimeApplyFailed, err
			}
			live = "candidate-live"
			live = "previous-live"
			return xraylive.RuntimeApplyFailed, persistErr
		},
		ack: func(context.Context, ownedhttp.Doer, clientEndpointRecord, int, string) error {
			acknowledged = true
			return nil
		},
	}
	candidate := clientInstallState{Endpoints: []clientEndpointRecord{{Tag: "rotation"}}}
	if _, _, err := runner.applySubscriptionCandidate(context.Background(), candidate, candidate.Endpoints[0], 0, 62022, "new-credential", true); !errors.Is(err, persistErr) {
		t.Fatalf("persistence error = %v, want %v", err, persistErr)
	}
	if acknowledged {
		t.Fatal("failed persistence acknowledged rotation")
	}
	if desired != "previous-desired" || live != "previous-live" {
		t.Fatalf("failed persistence changed Desired/Live: %q %q", desired, live)
	}
}

func TestSubscriptionCandidateRejectsMissingCredential(t *testing.T) {
	current := clientInstallState{Endpoints: []clientEndpointRecord{{Tag: "proxy-old", User: "alice"}}}
	_, err := subscriptionCandidate(current, current.Endpoints[0], controlplane.Subscription{
		Protocol: "trojan",
		Host:     "new.example",
		Port:     9443,
	}, "")
	if err == nil {
		t.Fatalf("expected missing credential error")
	}
}

func TestSubscriptionCandidateSwitchesToVLESSProfile(t *testing.T) {
	current := clientInstallState{Endpoints: []clientEndpointRecord{{
		Profile: "trojan-tls", Protocol: "trojan", Transport: "tcp", Security: "tls",
		Hostname: "edge.example", Address: "192.0.2.10", Tag: "proxy-edge", Port: 443, User: "alice",
	}}}
	sub := controlplane.Subscription{
		Generation: "vless-generation", Profile: "vless-tls-vision", Protocol: "vless", Transport: "tcp", Security: "tls",
		Host: "edge.example", Port: 443, ServerName: "edge.example", Parameters: map[string]string{"flow": "xtls-rprx-vision"},
	}
	candidate, err := subscriptionCandidate(current, current.Endpoints[0], sub, "550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("subscriptionCandidate: %v", err)
	}
	got := candidate.Endpoints[0]
	if got.Profile != "vless-tls-vision" || got.Protocol != "vless" || got.Flow != "xtls-rprx-vision" {
		t.Fatalf("profile was not applied: %+v", got)
	}
	if got.Address != "192.0.2.10" {
		t.Fatalf("resolved endpoint address was not preserved: %+v", got)
	}
}
