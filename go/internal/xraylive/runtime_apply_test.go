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

func TestTryApplyRoutingPendingReportsUnsupportedDiff(t *testing.T) {
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
	if result != RuntimeApplyUnsupported {
		t.Fatalf("result = %s, want %s", result, RuntimeApplyUnsupported)
	}
	if _, err := os.Stat(opts.RequestPath); err != nil {
		t.Fatalf("expected request to remain for service apply: %v", err)
	}
}

func TestTryApplyRoutingPendingAPIFailureDoesNotPublishLive(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	current := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"routing": {"rules": [{"type": "field", "ruleTag": "old", "outboundTag": "proxy-a"}]}
	}`)
	candidate := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"routing": {"rules": [{"type": "field", "ruleTag": "new", "outboundTag": "proxy-b"}]}
	}`)
	writeLive(t, opts.LiveDir, current, []byte(`{"version":1}`))
	opts.Compile = func(string, string) (Artifacts, error) {
		return Artifacts{XrayJSON: candidate, MetaJSON: []byte(`{"version":2}`)}, nil
	}
	opts.NewApplier = func(context.Context, string) (runtimeapply.RoutingApplier, func() error, error) {
		return nil, nil, errors.New("dial failed")
	}

	result, err := TryApplyRoutingPending(context.Background(), opts)
	if err != nil {
		t.Fatalf("TryApplyRoutingPending: %v", err)
	}
	if result != RuntimeApplyFailed {
		t.Fatalf("result = %s, want %s", result, RuntimeApplyFailed)
	}
	got, err := os.ReadFile(filepath.Join(opts.LiveDir, layout.XrayConfigFileName))
	if err != nil {
		t.Fatalf("read live xray: %v", err)
	}
	if string(got) != string(current) {
		t.Fatalf("live xray changed after API failure: %s", string(got))
	}
	if _, err := os.Stat(opts.RequestPath); err != nil {
		t.Fatalf("expected request to remain after API failure: %v", err)
	}
	if _, err := os.Stat(opts.ErrorPath); err != nil {
		t.Fatalf("expected apply error after API failure: %v", err)
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

func TestTryApplyRoutingPendingAppliesInboundDiff(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	current := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"inbounds": [
			{"tag": "keep", "protocol": "socks"},
			{"tag": "old", "protocol": "dokodemo-door"}
		]
	}`)
	candidate := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"inbounds": [
			{"tag": "keep", "protocol": "socks"},
			{"tag": "new", "protocol": "dokodemo-door"}
		]
	}`)
	writeLive(t, opts.LiveDir, current, []byte(`{"version":1}`))
	applier := newTestInboundApplier("keep", "old")
	opts.Compile = func(string, string) (Artifacts, error) {
		return Artifacts{XrayJSON: candidate, MetaJSON: []byte(`{"version":2}`)}, nil
	}
	opts.NewInbound = func(_ context.Context, address string) (runtimeapply.InboundApplier, func() error, error) {
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

func TestTryApplyRoutingPendingAppliesOutboundDiff(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	current := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"outbounds": [
			{"tag": "keep", "protocol": "freedom", "settings": {}},
			{"tag": "old", "protocol": "freedom", "settings": {}}
		]
	}`)
	candidate := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"outbounds": [
			{"tag": "keep", "protocol": "freedom", "settings": {}},
			{"tag": "new", "protocol": "freedom", "settings": {}}
		]
	}`)
	writeLive(t, opts.LiveDir, current, []byte(`{"version":1}`))
	applier := newTestOutboundApplier("keep", "old")
	opts.Compile = func(string, string) (Artifacts, error) {
		return Artifacts{XrayJSON: candidate, MetaJSON: []byte(`{"version":2}`)}, nil
	}
	opts.NewOutbound = func(_ context.Context, address string) (runtimeapply.OutboundApplier, func() error, error) {
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

func TestTryApplyRoutingPendingAppliesMixedRoutingOutboundDiff(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	current := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"outbounds": [
			{"tag": "direct", "protocol": "freedom", "settings": {}},
			{"tag": "proxy-old", "protocol": "freedom", "settings": {}}
		],
		"routing": {"rules": [
			{"type": "field", "ruleTag": "keep", "outboundTag": "direct"},
			{"type": "field", "ruleTag": "old-route", "outboundTag": "proxy-old"}
		]}
	}`)
	candidate := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"outbounds": [
			{"tag": "direct", "protocol": "freedom", "settings": {}},
			{"tag": "proxy-new", "protocol": "freedom", "settings": {}}
		],
		"routing": {"rules": [
			{"type": "field", "ruleTag": "keep", "outboundTag": "direct"},
			{"type": "field", "ruleTag": "new-route", "outboundTag": "proxy-new"}
		]}
	}`)
	writeLive(t, opts.LiveDir, current, []byte(`{"version":1}`))
	routingApplier := newTestRoutingApplier("keep", "old-route")
	outboundApplier := newTestOutboundApplier("direct", "proxy-old")
	opts.Compile = func(string, string) (Artifacts, error) {
		return Artifacts{XrayJSON: candidate, MetaJSON: []byte(`{"version":2}`)}, nil
	}
	opts.NewApplier = func(_ context.Context, address string) (runtimeapply.RoutingApplier, func() error, error) {
		if address != "127.0.0.1:10085" {
			t.Fatalf("routing address = %q", address)
		}
		return routingApplier, func() error { return nil }, nil
	}
	opts.NewOutbound = func(_ context.Context, address string) (runtimeapply.OutboundApplier, func() error, error) {
		if address != "127.0.0.1:10085" {
			t.Fatalf("outbound address = %q", address)
		}
		return outboundApplier, func() error { return nil }, nil
	}

	result, err := TryApplyRoutingPending(context.Background(), opts)
	if err != nil {
		t.Fatalf("TryApplyRoutingPending: %v", err)
	}
	if result != RuntimeApplyApplied {
		t.Fatalf("result = %s, want %s", result, RuntimeApplyApplied)
	}
	got, err := os.ReadFile(filepath.Join(opts.LiveDir, layout.XrayConfigFileName))
	if err != nil {
		t.Fatalf("read live xray: %v", err)
	}
	if string(got) != string(candidate) {
		t.Fatalf("live xray mismatch: %s", string(got))
	}
	if want := []string{"remove:old-route", "add:new-route"}; !reflect.DeepEqual(routingApplier.calls, want) {
		t.Fatalf("routing calls = %v, want %v", routingApplier.calls, want)
	}
	if want := []string{"remove:proxy-old", "add:proxy-new"}; !reflect.DeepEqual(outboundApplier.calls, want) {
		t.Fatalf("outbound calls = %v, want %v", outboundApplier.calls, want)
	}
}

func TestTryApplyRoutingPendingAppliesInboundUserDiff(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleServer)
	current := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"inbounds": [
			{
				"tag": "trojan-in",
				"protocol": "trojan",
				"settings": {"clients": [
					{"email": "old@example.com", "password": "old"}
				]}
			}
		]
	}`)
	candidate := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"inbounds": [
			{
				"tag": "trojan-in",
				"protocol": "trojan",
				"settings": {"clients": [
					{"email": "new@example.com", "password": "new"}
				]}
			}
		]
	}`)
	writeLive(t, opts.LiveDir, current, []byte(`{"version":1}`))
	applier := newTestInboundUserApplier()
	applier.users["trojan-in"] = map[string]string{"old@example.com": "old"}
	opts.Compile = func(string, string) (Artifacts, error) {
		return Artifacts{XrayJSON: candidate, MetaJSON: []byte(`{"version":2}`)}, nil
	}
	opts.NewInboundUser = func(_ context.Context, address string) (runtimeapply.InboundUserApplier, func() error, error) {
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
	got, err := os.ReadFile(filepath.Join(opts.LiveDir, layout.XrayConfigFileName))
	if err != nil {
		t.Fatalf("read live xray: %v", err)
	}
	if string(got) != string(candidate) {
		t.Fatalf("live xray mismatch: %s", string(got))
	}
	wantCalls := []string{"remove:trojan-in:old@example.com", "add:trojan-in:new@example.com"}
	if !reflect.DeepEqual(applier.calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", applier.calls, wantCalls)
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

type testInboundApplier struct {
	calls []string
	tags  map[string]struct{}
}

type testOutboundApplier struct {
	calls []string
	tags  map[string]struct{}
}

type testInboundUserApplier struct {
	calls []string
	users map[string]map[string]string
}

func newTestInboundUserApplier() *testInboundUserApplier {
	return &testInboundUserApplier{users: make(map[string]map[string]string)}
}

func (a *testInboundUserApplier) AddInboundUser(_ context.Context, inboundTag, email, password string) error {
	a.calls = append(a.calls, "add:"+inboundTag+":"+email)
	if a.users[inboundTag] == nil {
		a.users[inboundTag] = make(map[string]string)
	}
	a.users[inboundTag][email] = password
	return nil
}

func (a *testInboundUserApplier) RemoveInboundUser(_ context.Context, inboundTag, email string) error {
	a.calls = append(a.calls, "remove:"+inboundTag+":"+email)
	delete(a.users[inboundTag], email)
	return nil
}

func (a *testInboundUserApplier) ListInboundUserEmails(_ context.Context, inboundTag string) ([]string, error) {
	users := a.users[inboundTag]
	result := make([]string, 0, len(users))
	for email := range users {
		result = append(result, email)
	}
	return result, nil
}

func newTestOutboundApplier(tags ...string) *testOutboundApplier {
	applier := &testOutboundApplier{tags: make(map[string]struct{}, len(tags))}
	for _, tag := range tags {
		applier.tags[tag] = struct{}{}
	}
	return applier
}

func (a *testOutboundApplier) AddOutbound(_ context.Context, outbound map[string]any) error {
	tag, _ := outbound["tag"].(string)
	a.calls = append(a.calls, "add:"+tag)
	a.tags[tag] = struct{}{}
	return nil
}

func (a *testOutboundApplier) RemoveOutbound(_ context.Context, tag string) error {
	a.calls = append(a.calls, "remove:"+tag)
	delete(a.tags, tag)
	return nil
}

func (a *testOutboundApplier) ListOutboundTags(context.Context) ([]string, error) {
	result := make([]string, 0, len(a.tags))
	for tag := range a.tags {
		result = append(result, tag)
	}
	return result, nil
}

func newTestInboundApplier(tags ...string) *testInboundApplier {
	applier := &testInboundApplier{tags: make(map[string]struct{}, len(tags))}
	for _, tag := range tags {
		applier.tags[tag] = struct{}{}
	}
	return applier
}

func (a *testInboundApplier) AddInbound(_ context.Context, inbound map[string]any) error {
	tag, _ := inbound["tag"].(string)
	a.calls = append(a.calls, "add:"+tag)
	a.tags[tag] = struct{}{}
	return nil
}

func (a *testInboundApplier) RemoveInbound(_ context.Context, tag string) error {
	a.calls = append(a.calls, "remove:"+tag)
	delete(a.tags, tag)
	return nil
}

func (a *testInboundApplier) ListInboundTags(context.Context) ([]string, error) {
	result := make([]string, 0, len(a.tags))
	for tag := range a.tags {
		result = append(result, tag)
	}
	return result, nil
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
