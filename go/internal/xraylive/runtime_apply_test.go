package xraylive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/runtimeapply"
)

func TestTryApplyRoutingPendingAppliesAndPublishesLive(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	current := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"routing": {"rules": [
			{"type": "field", "ruleTag": "keep", "outboundTag": "direct"},
			{"type": "field", "ruleTag": "old", "outboundTag": "proxy-a"}
		]}
	}`)
	candidate := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"routing": {"rules": [
			{"type": "field", "ruleTag": "keep", "outboundTag": "direct"},
			{"type": "field", "ruleTag": "new", "outboundTag": "proxy-b"}
		]}
	}`)
	writeLive(t, opts.LiveDir, current, []byte(`{"version":1}`))
	applier := newTestRoutingApplier("keep", "old")
	opts.Compile = func(string, string) (Artifacts, error) {
		return Artifacts{XrayJSON: candidate, MetaJSON: []byte(`{"version":2}`)}, nil
	}
	opts.NewApplier = func(_ context.Context, address string) (runtimeapply.RoutingApplier, func() error, error) {
		if address != "127.0.0.1:10085" {
			t.Fatalf("address = %q", address)
		}
		return applier, func() error { return nil }, nil
	}

	result, err := TryApplyRoutingPending(context.Background(), opts)
	if err != nil {
		t.Fatalf("TryApplyRoutingPending: %v", err)
	}
	if result != RuntimeApplyApplied {
		t.Fatalf("result = %s, want %s", result, RuntimeApplyApplied)
	}
	if _, err := os.Stat(opts.RequestPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected request removed, stat err=%v", err)
	}
	got, err := os.ReadFile(filepath.Join(opts.LiveDir, layout.XrayConfigFileName))
	if err != nil {
		t.Fatalf("read live xray: %v", err)
	}
	if string(got) != string(candidate) {
		t.Fatalf("live xray mismatch: %s", string(got))
	}
	wantCalls := []string{"remove:old", "add:new"}
	if !reflect.DeepEqual(applier.calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", applier.calls, wantCalls)
	}
}

func TestTryApplyRoutingPendingFallsBackForUnsupportedDiff(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	writeLive(t, opts.LiveDir,
		[]byte(`{"log":{"loglevel":"warning"},"routing":{"rules":[{"ruleTag":"a"}]}}`),
		[]byte(`{"version":1}`),
	)
	opts.Compile = func(string, string) (Artifacts, error) {
		return Artifacts{
			XrayJSON: []byte(`{"log":{"loglevel":"debug"},"routing":{"rules":[{"ruleTag":"a"}]}}`),
			MetaJSON: []byte(`{"version":2}`),
		}, nil
	}

	result, err := TryApplyRoutingPending(context.Background(), opts)
	if err != nil {
		t.Fatalf("TryApplyRoutingPending: %v", err)
	}
	if result != RuntimeApplyRestartRequired {
		t.Fatalf("result = %s, want %s", result, RuntimeApplyRestartRequired)
	}
	if _, err := os.Stat(opts.RequestPath); err != nil {
		t.Fatalf("expected request to remain for restart fallback: %v", err)
	}
}

func TestTryApplyRoutingPendingPublishesNoopWithoutAPI(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	current := []byte(`{"routing":{"rules":[{"ruleTag":"a"}]}}`)
	writeLive(t, opts.LiveDir, current, []byte(`{"version":1}`))
	opts.Compile = func(string, string) (Artifacts, error) {
		return Artifacts{XrayJSON: current, MetaJSON: []byte(`{"version":2}`)}, nil
	}
	opts.NewApplier = func(context.Context, string) (runtimeapply.RoutingApplier, func() error, error) {
		t.Fatal("unexpected API call for noop diff")
		return nil, nil, nil
	}

	result, err := TryApplyRoutingPending(context.Background(), opts)
	if err != nil {
		t.Fatalf("TryApplyRoutingPending: %v", err)
	}
	if result != RuntimeApplyNoop {
		t.Fatalf("result = %s, want %s", result, RuntimeApplyNoop)
	}
	meta, err := os.ReadFile(filepath.Join(opts.LiveDir, layout.RuntimeMetaFileName))
	if err != nil {
		t.Fatalf("read live meta: %v", err)
	}
	if string(meta) != `{"version":2}` {
		t.Fatalf("live meta mismatch: %s", string(meta))
	}
}

func testOptions(t *testing.T, root, role string) Options {
	t.Helper()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	stateDir := filepath.Join(root, layout.StateDirName)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	req, err := apply.NewRequest(role)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	requestPath := filepath.Join(stateDir, layout.ApplyRequestFileName)
	if err := apply.WriteRequest(requestPath, req, ""); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}
	liveDir, err := config.LiveRoleDir(role)
	if err != nil {
		t.Fatalf("LiveRoleDir: %v", err)
	}
	lkgDir, err := config.LkgRoleDir(role)
	if err != nil {
		t.Fatalf("LkgRoleDir: %v", err)
	}
	return Options{
		Role:        role,
		RequestPath: requestPath,
		ErrorPath:   filepath.Join(stateDir, layout.ApplyErrorFileName),
		LiveDir:     liveDir,
		LkgDir:      lkgDir,
	}
}

func writeLive(t *testing.T, liveDir string, xrayJSON, metaJSON []byte) {
	t.Helper()
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("mkdir live: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, layout.XrayConfigFileName), xrayJSON, 0o644); err != nil {
		t.Fatalf("write xray: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, layout.RuntimeMetaFileName), metaJSON, 0o644); err != nil {
		t.Fatalf("write meta: %v", err)
	}
}

type testRoutingApplier struct {
	calls []string
	tags  map[string]struct{}
}

func newTestRoutingApplier(tags ...string) *testRoutingApplier {
	applier := &testRoutingApplier{tags: make(map[string]struct{}, len(tags))}
	for _, tag := range tags {
		applier.tags[tag] = struct{}{}
	}
	return applier
}

func (a *testRoutingApplier) AddRule(_ context.Context, rule map[string]any) error {
	tag, _ := rule["ruleTag"].(string)
	a.calls = append(a.calls, "add:"+tag)
	a.tags[tag] = struct{}{}
	return nil
}

func (a *testRoutingApplier) RemoveRule(_ context.Context, tag string) error {
	a.calls = append(a.calls, "remove:"+tag)
	delete(a.tags, tag)
	return nil
}

func (a *testRoutingApplier) ListRuleTags(context.Context) ([]string, error) {
	result := make([]string, 0, len(a.tags))
	for tag := range a.tags {
		result = append(result, tag)
	}
	return result, nil
}
