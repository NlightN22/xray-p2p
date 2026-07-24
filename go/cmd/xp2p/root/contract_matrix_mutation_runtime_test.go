package root

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/runtimeboundary"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

var nonRuntimeMutationReasons = map[string]string{
	"xp2p server ha peer add":     "peer transport trust is control-plane state, not an Xray runtime resource",
	"xp2p server ha peer remove":  "peer transport trust is control-plane state, not an Xray runtime resource",
	"xp2p server ha peer self":    "the local peer ID is control-plane state, not an Xray runtime resource",
	"xp2p server identity detach": "provider selection is identity control state; sync applies runtime resources",
	"xp2p server identity select": "provider selection is identity control state; sync applies runtime resources",
}

type runtimeMutationRecorder struct {
	applyCalls   int
	statusCalls  int
	restartCalls int
	runtime      xraylive.Artifacts
	fail         bool
}

type roleMutationState struct {
	desired []byte
	live    map[string][]byte
	request []byte
}

func TestStage3RuntimeApplyContractCases(t *testing.T) {
	paths := make([]string, 0, len(mutationContractRegistry))
	for path := range mutationContractRegistry {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		path := path
		scenario := mutationContractRegistry[path]
		if reason := nonRuntimeMutationReasons[path]; reason != "" {
			t.Run(path+"/not-api-capable", func(t *testing.T) {
				if reason == "" {
					t.Fatal("non-runtime mutation requires an explicit reason")
				}
			})
			continue
		}
		t.Run(path+"/runtime-success", func(t *testing.T) {
			fixture := scenario.successFixture(t)
			role := mutationRole(path)
			beforeDomain := fixture.snapshot(t)
			recorder := &runtimeMutationRecorder{}
			restore := runtimeboundary.SetForTesting(recorder.boundary())
			t.Cleanup(restore)

			execution := executeContractCase(fixture.args, false)
			assertMutationSuccess(t, path, execution, fixture.sensitive, fixture.expectedEntity)
			after := snapshotRoleMutationState(t, role)
			afterDomain := fixture.snapshot(t)

			if recorder.applyCalls != 1 {
				t.Fatalf("runtime apply calls=%d, want 1", recorder.applyCalls)
			}
			if reflect.DeepEqual(beforeDomain, afterDomain) {
				t.Fatal("runtime success did not commit the command's Desired/control state")
			}
			assertRuntimeMatchesLive(t, recorder.runtime, after.live)
			if len(after.request) != 0 {
				t.Fatalf("runtime success created apply.request: %q", after.request)
			}
			if recorder.restartCalls != 0 {
				t.Fatalf("runtime success used restart fallback %d times", recorder.restartCalls)
			}
		})
		t.Run(path+"/runtime-failure-atomic", func(t *testing.T) {
			fixture := scenario.successFixture(t)
			role := mutationRole(path)
			before := snapshotRoleMutationState(t, role)
			recorder := &runtimeMutationRecorder{fail: true}
			restore := runtimeboundary.SetForTesting(recorder.boundary())
			t.Cleanup(restore)

			execution := executeContractCase(fixture.args, false)
			assertMutationFailure(t, path, execution, fixture.sensitive)
			after := snapshotRoleMutationState(t, role)

			if recorder.applyCalls != 1 {
				t.Fatalf("runtime apply calls=%d, want 1", recorder.applyCalls)
			}
			if recorder.statusCalls != 1 {
				t.Fatalf("service status calls=%d, want 1", recorder.statusCalls)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("runtime failure changed Desired/Live/apply.request:\nbefore=%#v\nafter=%#v", before, after)
			}
			if recorder.restartCalls != 0 {
				t.Fatalf("runtime failure used restart fallback %d times", recorder.restartCalls)
			}
		})
	}
}

func (r *runtimeMutationRecorder) boundary() runtimeboundary.Boundary {
	return runtimeboundary.Boundary{
		ApplyCandidate: func(
			_ context.Context,
			opts xraylive.Options,
			artifacts xraylive.Artifacts,
		) (xraylive.RuntimeApplyResult, error) {
			r.applyCalls++
			if r.fail {
				return xraylive.RuntimeApplyFailed, errors.New("runtime API fixture rejected candidate")
			}
			r.runtime = artifacts
			if err := writeRuntimeFixture(opts.LiveDir, artifacts); err != nil {
				return xraylive.RuntimeApplyFailed, err
			}
			if opts.CommitDesired != nil {
				if err := opts.CommitDesired(); err != nil {
					return xraylive.RuntimeApplyFailed, err
				}
			}
			return xraylive.RuntimeApplyApplied, nil
		},
		ServiceStatus: func(_ context.Context, _ servicecontrol.Role) (servicecontrol.Status, error) {
			r.statusCalls++
			return servicecontrol.Status{Active: true, State: "running"}, nil
		},
		RestartService: func(_ context.Context, _ servicecontrol.Role) error {
			r.restartCalls++
			return nil
		},
	}
}

func writeRuntimeFixture(liveDir string, artifacts xraylive.Artifacts) error {
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		return err
	}
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
			return err
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func mutationRole(path string) string {
	if len(path) >= len("xp2p client ") && path[:len("xp2p client ")] == "xp2p client " {
		return apply.RoleClient
	}
	return apply.RoleServer
}

func snapshotRoleMutationState(t *testing.T, role string) roleMutationState {
	t.Helper()
	fileName := layout.ServerConfigFileName
	if role == apply.RoleClient {
		fileName = layout.ClientConfigFileName
	}
	desired, err := os.ReadFile(config.ConfigPath(fileName))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	liveDir, err := config.LiveRoleDir(role)
	if err != nil {
		t.Fatal(err)
	}
	request, err := os.ReadFile(config.ApplyRequestPath())
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return roleMutationState{
		desired: desired,
		live:    snapshotDirectory(t, liveDir),
		request: request,
	}
}

func snapshotDirectory(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.Clean(relative)] = data
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return files
}

func assertRuntimeMatchesLive(t *testing.T, runtime xraylive.Artifacts, live map[string][]byte) {
	t.Helper()
	if !reflect.DeepEqual(live[layout.XrayConfigFileName], runtime.XrayJSON) {
		t.Fatal("running Xray config and Live xray config differ")
	}
	if !reflect.DeepEqual(live[layout.RuntimeMetaFileName], runtime.MetaJSON) {
		t.Fatal("running runtime metadata and Live metadata differ")
	}
	for name, data := range runtime.Extra {
		if !reflect.DeepEqual(live[filepath.Clean(name)], data) {
			t.Fatalf("running artifact %s and Live artifact differ", name)
		}
	}
}
