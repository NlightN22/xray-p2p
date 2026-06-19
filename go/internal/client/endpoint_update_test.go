//go:build linux || windows

package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func TestUpdateEndpointCredentialsAppliesRuntimeWhenServiceIsRunning(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	statePath := writeEndpointUpdateState(t)
	liveDir := writeClientLive(t, "old-live")
	var applied bool
	stubRuntimeFlow(t, true, func(_ context.Context, opts xraylive.Options, artifacts xraylive.Artifacts) (xraylive.RuntimeApplyResult, error) {
		applied = true
		if opts.Role != apply.RoleClient {
			t.Fatalf("runtime role = %q", opts.Role)
		}
		lkgDir, err := config.LkgRoleDir(apply.RoleClient)
		if err != nil {
			t.Fatalf("LkgRoleDir: %v", err)
		}
		if err := apply.ReplaceRoleLiveDir(liveDir, lkgDir, map[string][]byte{
			layout.XrayConfigFileName:  artifacts.XrayJSON,
			layout.RuntimeMetaFileName: artifacts.MetaJSON,
		}); err != nil {
			t.Fatalf("publish live: %v", err)
		}
		return xraylive.RuntimeApplyApplied, nil
	})

	if err := updateEndpointForTest(); err != nil {
		t.Fatalf("UpdateEndpointCredentials failed: %v", err)
	}

	if !applied {
		t.Fatal("runtime API apply was not attempted")
	}
	assertEndpointCredentials(t, statePath, "new-user", "new-password")
	live, err := os.ReadFile(filepath.Join(liveDir, layout.XrayConfigFileName))
	if err != nil {
		t.Fatalf("read live xray: %v", err)
	}
	if !strings.Contains(string(live), "new-user") {
		t.Fatalf("live xray was not updated by runtime apply: %s", string(live))
	}
	assertNoApplyRequest(t)
}

func TestUpdateEndpointCredentialsStagesOnlyDesiredWhenServiceIsStopped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	statePath := writeEndpointUpdateState(t)
	liveDir := writeClientLive(t, "old-live")
	beforeLive := readFile(t, filepath.Join(liveDir, layout.XrayConfigFileName))
	stubRuntimeFlow(t, false, func(context.Context, xraylive.Options, xraylive.Artifacts) (xraylive.RuntimeApplyResult, error) {
		t.Fatal("runtime API apply should not be called for stopped service")
		return xraylive.RuntimeApplySkipped, nil
	})

	if err := updateEndpointForTest(); err != nil {
		t.Fatalf("UpdateEndpointCredentials failed: %v", err)
	}

	assertEndpointCredentials(t, statePath, "new-user", "new-password")
	afterLive := readFile(t, filepath.Join(liveDir, layout.XrayConfigFileName))
	if string(afterLive) != string(beforeLive) {
		t.Fatalf("live xray changed while service was stopped: %s", string(afterLive))
	}
	assertNoApplyRequest(t)
}

func TestUpdateEndpointCredentialsFailsWithoutChangingStateWhenRuntimeAPIFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	statePath := writeEndpointUpdateState(t)
	liveDir := writeClientLive(t, "old-live")
	beforeDesired := readFile(t, statePath)
	beforeLive := readFile(t, filepath.Join(liveDir, layout.XrayConfigFileName))
	stubRuntimeFlow(t, true, func(context.Context, xraylive.Options, xraylive.Artifacts) (xraylive.RuntimeApplyResult, error) {
		return xraylive.RuntimeApplyFailed, errors.New("dial xray API: connection refused")
	})

	err := updateEndpointForTest()
	if err == nil {
		t.Fatal("UpdateEndpointCredentials succeeded after runtime API failure")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("error does not include API failure reason: %v", err)
	}
	afterDesired := readFile(t, statePath)
	afterLive := readFile(t, filepath.Join(liveDir, layout.XrayConfigFileName))
	if string(afterDesired) != string(beforeDesired) {
		t.Fatalf("desired state changed after runtime API failure:\n%s", string(afterDesired))
	}
	if string(afterLive) != string(beforeLive) {
		t.Fatalf("live xray changed after runtime API failure: %s", string(afterLive))
	}
	assertNoApplyRequest(t)
}

func writeEndpointUpdateState(t *testing.T) string {
	t.Helper()

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	initial := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{
				Hostname:      "edge.example",
				Tag:           "proxy-edge",
				Address:       "198.51.100.10",
				Port:          8443,
				User:          "old-user",
				Password:      "old-password",
				ServerName:    "edge.example",
				AllowInsecure: true,
			},
		},
		Redirects: []redirect.Rule{{CIDR: "10.20.0.0/16", OutboundTag: "proxy-edge"}},
		Reverse: map[string]clientReverseChannel{
			"oldedge-example.rev": {
				UserID:      "old-user",
				Host:        "edge.example",
				Tag:         "oldedge-example.rev",
				Domain:      "oldedge-example.rev",
				EndpointTag: "proxy-edge",
			},
		},
	}
	if err := initial.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}
	return statePath
}

func updateEndpointForTest() error {
	return UpdateEndpointCredentials(context.Background(), UpdateEndpointOptions{
		Target:      "proxy-edge",
		User:        "new-user",
		Password:    "new-password",
		UserSet:     true,
		PasswordSet: true,
	})
}

func assertEndpointCredentials(t *testing.T, statePath, user, password string) {
	t.Helper()
	updated, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if got := len(updated.Endpoints); got != 1 {
		t.Fatalf("expected one endpoint, got %d", got)
	}
	ep := updated.Endpoints[0]
	if ep.User != user || ep.Password != password {
		t.Fatalf("credentials were not staged: %+v", ep)
	}
	if ep.Tag != "proxy-edge" || ep.Hostname != "edge.example" || ep.Address != "198.51.100.10" {
		t.Fatalf("immutable endpoint fields changed: %+v", ep)
	}
	if len(updated.Redirects) != 1 || updated.Redirects[0].OutboundTag != "proxy-edge" {
		t.Fatalf("redirects changed: %+v", updated.Redirects)
	}
	channel := updated.Reverse["oldedge-example.rev"]
	if channel.UserID != "old-user" || channel.EndpointTag != "proxy-edge" {
		t.Fatalf("reverse channel changed: %+v", channel)
	}
}

func assertNoApplyRequest(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(config.ApplyRequestPath()); !os.IsNotExist(err) {
		t.Fatalf("apply request should not be written for runtime command: %v", err)
	}
}

func writeClientLive(t *testing.T, marker string) string {
	t.Helper()
	liveDir, err := config.LiveRoleDir(apply.RoleClient)
	if err != nil {
		t.Fatalf("LiveRoleDir: %v", err)
	}
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("mkdir live: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, layout.XrayConfigFileName), []byte(marker), 0o644); err != nil {
		t.Fatalf("write live xray: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, layout.RuntimeMetaFileName), []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatalf("write live meta: %v", err)
	}
	return liveDir
}

func stubRuntimeFlow(
	t *testing.T,
	serviceActive bool,
	applyFunc func(context.Context, xraylive.Options, xraylive.Artifacts) (xraylive.RuntimeApplyResult, error),
) {
	t.Helper()
	oldStatus := serviceStatus
	oldApply := applyRuntimeCandidate
	serviceStatus = func(context.Context, servicecontrol.Role) (servicecontrol.Status, error) {
		return servicecontrol.Status{Active: serviceActive}, nil
	}
	applyRuntimeCandidate = applyFunc
	t.Cleanup(func() {
		serviceStatus = oldStatus
		applyRuntimeCandidate = oldApply
	})
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
