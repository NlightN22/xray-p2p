package root

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	clientcore "github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/runtimeapply"
	"github.com/NlightN22/xray-p2p/go/internal/runtimeboundary"
	servercore "github.com/NlightN22/xray-p2p/go/internal/server"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

type runtimeAPIMode int

const (
	runtimeAPISuccess runtimeAPIMode = iota
	runtimeAPIFailVerification
	runtimeAPIFailDesiredCommit
)

type runtimeAPIRecorder struct {
	mode          runtimeAPIMode
	role          string
	initial       runtimeAPIState
	current       runtimeAPIState
	factoryCalls  int
	mutationCalls int
	statusCalls   int
	restartCalls  int
	failureSent   bool
	desiredPath   string
	desiredBackup string
}

func newRuntimeAPIRecorder(t *testing.T, path, role string, mode runtimeAPIMode) *runtimeAPIRecorder {
	t.Helper()
	if path == "xp2p server identity sync" {
		seedIdentityRuntimeState(t)
	}
	artifacts := compileRoleDesired(t, role)
	liveDir, err := config.LiveRoleDir(role)
	if err != nil {
		t.Fatal(err)
	}
	writeRuntimeArtifacts(t, liveDir, artifacts)
	state := parseRuntimeAPIState(t, artifacts.XrayJSON)
	return &runtimeAPIRecorder{
		mode:          mode,
		role:          role,
		initial:       state.clone(),
		current:       state,
		desiredPath:   roleDesiredPath(role),
		desiredBackup: roleDesiredPath(role) + ".runtime-contract-backup",
	}
}

func (r *runtimeAPIRecorder) boundary() runtimeboundary.Boundary {
	factory := func() { r.factoryCalls++ }
	return runtimeboundary.Boundary{
		NewRouting: func(context.Context, string) (runtimeapply.RoutingApplier, func() error, error) {
			factory()
			return r, func() error { return nil }, nil
		},
		NewInbound: func(context.Context, string) (runtimeapply.InboundApplier, func() error, error) {
			factory()
			return r, func() error { return nil }, nil
		},
		NewOutbound: func(context.Context, string) (runtimeapply.OutboundApplier, func() error, error) {
			factory()
			return r, func() error { return nil }, nil
		},
		NewInboundUser: func(context.Context, string) (runtimeapply.InboundUserApplier, func() error, error) {
			factory()
			return r, func() error { return nil }, nil
		},
		ServiceStatus: func(context.Context, servicecontrol.Role) (servicecontrol.Status, error) {
			r.statusCalls++
			return servicecontrol.Status{Active: true, State: "running"}, nil
		},
		RestartService: func(context.Context, servicecontrol.Role) error {
			r.restartCalls++
			return nil
		},
	}
}

func (r *runtimeAPIRecorder) assertProductionFlow(t *testing.T, role string, success bool) {
	t.Helper()
	if r.factoryCalls == 0 || r.mutationCalls == 0 {
		t.Fatalf("production runtime apply did not reach the Xray API client: factories=%d mutations=%d", r.factoryCalls, r.mutationCalls)
	}
	if r.restartCalls != 0 {
		t.Fatalf("runtime flow used restart fallback %d times", r.restartCalls)
	}
	if success {
		artifacts := compileRoleDesired(t, role)
		liveDir, err := config.LiveRoleDir(role)
		if err != nil {
			t.Fatal(err)
		}
		live := snapshotDirectory(t, liveDir)
		assertCompiledArtifactsMatchLive(t, artifacts, live)
		wantRuntime := parseRuntimeAPIState(t, artifacts.XrayJSON)
		if !reflect.DeepEqual(r.current, wantRuntime) {
			t.Fatalf("Runtime and compiled Desired/Live differ:\nruntime=%#v\ncompiled=%#v", r.current, wantRuntime)
		}
		return
	}
	if !reflect.DeepEqual(r.current, r.initial) {
		t.Fatalf("runtime rollback did not restore initial state:\nbefore=%#v\nafter=%#v", r.initial, r.current)
	}
	if r.statusCalls != 1 {
		t.Fatalf("runtime failure service status calls=%d, want 1", r.statusCalls)
	}
}

func (r *runtimeAPIRecorder) verificationFailure() error {
	if r.mutationCalls == 0 || r.failureSent {
		return nil
	}
	switch r.mode {
	case runtimeAPIFailVerification:
		r.failureSent = true
		return errors.New("runtime API verification fixture failed")
	case runtimeAPIFailDesiredCommit:
		r.failureSent = true
		if err := os.Rename(r.desiredPath, r.desiredBackup); err != nil {
			return err
		}
		return os.Mkdir(r.desiredPath, 0o700)
	default:
		return nil
	}
}

func (r *runtimeAPIRecorder) restoreDesired(t *testing.T) {
	t.Helper()
	if r.mode != runtimeAPIFailDesiredCommit || !r.failureSent {
		return
	}
	if err := os.RemoveAll(r.desiredPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(r.desiredBackup, r.desiredPath); err != nil {
		t.Fatal(err)
	}
}

func (r *runtimeAPIRecorder) AddRule(_ context.Context, rule map[string]any) error {
	r.mutationCalls++
	r.current.rules[stringField(rule, "ruleTag")] = cloneObject(rule)
	return nil
}

func (r *runtimeAPIRecorder) RemoveRule(_ context.Context, tag string) error {
	r.mutationCalls++
	delete(r.current.rules, tag)
	return nil
}

func (r *runtimeAPIRecorder) ListRuleTags(context.Context) ([]string, error) {
	if err := r.verificationFailure(); err != nil {
		return nil, err
	}
	return sortedKeys(r.current.rules), nil
}

func (r *runtimeAPIRecorder) AddInbound(_ context.Context, inbound map[string]any) error {
	r.mutationCalls++
	r.current.addInbound(inbound)
	return nil
}

func (r *runtimeAPIRecorder) RemoveInbound(_ context.Context, tag string) error {
	r.mutationCalls++
	delete(r.current.inbounds, tag)
	delete(r.current.users, tag)
	return nil
}

func (r *runtimeAPIRecorder) ListInboundTags(context.Context) ([]string, error) {
	if err := r.verificationFailure(); err != nil {
		return nil, err
	}
	return sortedKeys(r.current.inbounds), nil
}

func (r *runtimeAPIRecorder) AddOutbound(_ context.Context, outbound map[string]any) error {
	r.mutationCalls++
	r.current.outbounds[stringField(outbound, "tag")] = cloneObject(outbound)
	return nil
}

func (r *runtimeAPIRecorder) RemoveOutbound(_ context.Context, tag string) error {
	r.mutationCalls++
	delete(r.current.outbounds, tag)
	return nil
}

func (r *runtimeAPIRecorder) ListOutboundTags(context.Context) ([]string, error) {
	if err := r.verificationFailure(); err != nil {
		return nil, err
	}
	return sortedKeys(r.current.outbounds), nil
}

func (r *runtimeAPIRecorder) AddInboundUser(_ context.Context, tag, email, password string) error {
	r.mutationCalls++
	if r.current.users[tag] == nil {
		r.current.users[tag] = make(map[string]runtimeAPIUser)
	}
	key := strings.ToLower(email)
	r.current.users[tag][key] = runtimeAPIUser{Email: key, Password: password}
	return nil
}

func (r *runtimeAPIRecorder) RemoveInboundUser(_ context.Context, tag, email string) error {
	r.mutationCalls++
	delete(r.current.users[tag], strings.ToLower(email))
	if len(r.current.users[tag]) == 0 {
		delete(r.current.users, tag)
	}
	return nil
}

func (r *runtimeAPIRecorder) ListInboundUserEmails(_ context.Context, tag string) ([]string, error) {
	if err := r.verificationFailure(); err != nil {
		return nil, err
	}
	return sortedKeys(r.current.users[tag]), nil
}

func writeRuntimeArtifacts(t *testing.T, liveDir string, artifacts xraylive.Artifacts) {
	t.Helper()
	files := map[string][]byte{
		layout.XrayConfigFileName:  artifacts.XrayJSON,
		layout.RuntimeMetaFileName: artifacts.MetaJSON,
	}
	for name, data := range artifacts.Extra {
		files[filepath.Clean(name)] = data
	}
	for name, data := range files {
		path := filepath.Join(liveDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func compileRoleDesired(t *testing.T, role string) xraylive.Artifacts {
	t.Helper()
	extensions, err := config.DesiredExtensionsDirForRole(role)
	if err != nil {
		t.Fatal(err)
	}
	if role == apply.RoleClient {
		value, err := clientcore.CompileDesiredArtifacts(roleDesiredPath(role), extensions)
		if err != nil {
			t.Fatal(err)
		}
		return xraylive.Artifacts{XrayJSON: value.XrayJSON, MetaJSON: value.RuntimeMetaJSON}
	}
	value, err := servercore.CompileDesiredArtifacts(roleDesiredPath(role), extensions)
	if err != nil {
		t.Fatal(err)
	}
	return xraylive.Artifacts{XrayJSON: value.XrayJSON, MetaJSON: value.RuntimeMetaJSON, Extra: value.Extra}
}

func assertCompiledArtifactsMatchLive(t *testing.T, artifacts xraylive.Artifacts, live map[string][]byte) {
	t.Helper()
	if !reflect.DeepEqual(live[layout.XrayConfigFileName], artifacts.XrayJSON) {
		t.Fatal("compiled Desired xray config and Live xray config differ")
	}
	if !reflect.DeepEqual(
		normalizedRuntimeMeta(t, live[layout.RuntimeMetaFileName]),
		normalizedRuntimeMeta(t, artifacts.MetaJSON),
	) {
		t.Fatal("compiled Desired runtime metadata and Live metadata differ")
	}
	for name, data := range artifacts.Extra {
		if !reflect.DeepEqual(live[filepath.Clean(name)], data) {
			t.Fatalf("compiled Desired artifact %s and Live artifact differ", name)
		}
	}
}
