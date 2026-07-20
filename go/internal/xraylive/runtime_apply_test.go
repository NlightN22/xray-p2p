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
	"github.com/NlightN22/xray-p2p/go/internal/xrayapi"
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

func TestTryApplyRoutingPendingAppliesSocksInboundReplacement(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	current := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"inbounds": [
			{"tag": "socks-in", "listen": "127.0.0.1", "port": 51180, "protocol": "socks", "settings": {"udp": true}}
		]
	}`)
	candidate := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"inbounds": [
			{"tag": "socks-in", "listen": "0.0.0.0", "port": 51180, "protocol": "socks", "settings": {"udp": true}}
		]
	}`)
	writeLive(t, opts.LiveDir, current, []byte(`{"role":"client","tun_enabled":false}`))
	applier := newTestInboundApplier("socks-in")
	opts.Compile = func(string, string) (Artifacts, error) {
		return Artifacts{XrayJSON: candidate, MetaJSON: []byte(`{"role":"client","tun_enabled":false}`)}, nil
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
	wantCalls := []string{"remove:socks-in", "add:socks-in"}
	if !reflect.DeepEqual(applier.calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", applier.calls, wantCalls)
	}
}

func TestTryApplyRoutingPendingRequiresServiceLayerForTunModeChange(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	current := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"inbounds": [
			{"tag": "tun-in", "protocol": "tun"},
			{"tag": "socks-in", "protocol": "socks"}
		]
	}`)
	candidate := []byte(`{
		"api": {"listen": "127.0.0.1:10085"},
		"inbounds": [
			{"tag": "socks-in", "protocol": "socks"}
		]
	}`)
	writeLive(t, opts.LiveDir, current, []byte(`{"role":"client","tun_enabled":true,"tun_name":"xp2pc"}`))
	opts.Compile = func(string, string) (Artifacts, error) {
		return Artifacts{
			XrayJSON: candidate,
			MetaJSON: []byte(`{"role":"client","tun_enabled":false,"tun_name":"xp2pc"}`),
		}, nil
	}
	opts.NewInbound = func(context.Context, string) (runtimeapply.InboundApplier, func() error, error) {
		t.Fatal("unexpected runtime inbound apply for tun mode change")
		return nil, nil, nil
	}

	result, err := TryApplyRoutingPending(context.Background(), opts)
	if err != nil {
		t.Fatalf("TryApplyRoutingPending: %v", err)
	}
	if result != RuntimeApplyServiceLayerRequired {
		t.Fatalf("result = %s, want %s", result, RuntimeApplyServiceLayerRequired)
	}
	got, err := os.ReadFile(filepath.Join(opts.LiveDir, layout.XrayConfigFileName))
	if err != nil {
		t.Fatalf("read live xray: %v", err)
	}
	if string(got) != string(current) {
		t.Fatalf("live xray changed after service-layer decision: %s", string(got))
	}
	if _, err := os.Stat(opts.RequestPath); err != nil {
		t.Fatalf("expected request to remain for service restart: %v", err)
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

func TestTryApplyRoutingPendingRollsBackWhenRuntimeVerificationFails(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleServer)
	current := []byte(`{"api":{"listen":"127.0.0.1:10085"},"inbounds":[{"tag":"trojan-in","protocol":"trojan","settings":{"clients":[{"email":"old@example.com","password":"old"}]}}]}`)
	candidate := []byte(`{"api":{"listen":"127.0.0.1:10085"},"inbounds":[{"tag":"trojan-in","protocol":"trojan","settings":{"clients":[{"email":"new@example.com","password":"new"}]}}]}`)
	writeLive(t, opts.LiveDir, current, []byte(`{"version":1}`))
	applier := newTestInboundUserApplier()
	applier.users["trojan-in"] = map[string]string{"old@example.com": "old"}
	opts.Compile = func(string, string) (Artifacts, error) {
		return Artifacts{XrayJSON: candidate, MetaJSON: []byte(`{"version":2}`)}, nil
	}
	opts.NewInboundUser = func(context.Context, string) (runtimeapply.InboundUserApplier, func() error, error) {
		return applier, func() error { return nil }, nil
	}
	opts.VerifyRuntime = func(context.Context) error { return errors.New("tunnel probe failed") }

	result, err := TryApplyRoutingPending(context.Background(), opts)
	if err != nil {
		t.Fatalf("TryApplyRoutingPending: %v", err)
	}
	if result != RuntimeApplyFailed {
		t.Fatalf("result = %s, want %s", result, RuntimeApplyFailed)
	}
	got, err := os.ReadFile(filepath.Join(opts.LiveDir, layout.XrayConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(current) {
		t.Fatalf("live changed after failed verification: %s", got)
	}
	wantCalls := []string{"remove:trojan-in:old@example.com", "add:trojan-in:new@example.com", "remove:trojan-in:new@example.com", "add:trojan-in:old@example.com"}
	if !reflect.DeepEqual(applier.calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", applier.calls, wantCalls)
	}
}

func TestTryApplyRoutingPendingPreservesCurrentAPIListen(t *testing.T) {
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
		"api": {"listen": "127.0.0.1:20085"},
		"inbounds": [
			{
				"tag": "trojan-in",
				"protocol": "trojan",
				"settings": {"clients": []}
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
	listen, err := xrayapi.APIListenFromConfig(got)
	if err != nil {
		t.Fatalf("read api listen: %v", err)
	}
	if listen != "127.0.0.1:10085" {
		t.Fatalf("api listen = %q, want current listen", listen)
	}
	wantCalls := []string{"remove:trojan-in:old@example.com"}
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

func TestApplyCandidateDoesNotCompleteQueuedRequest(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	xrayJSON := []byte(`{"api":{"listen":"127.0.0.1:10085"}}`)
	metaJSON := []byte(`{"role":"client"}`)
	writeLive(t, opts.LiveDir, xrayJSON, metaJSON)

	result, err := ApplyCandidate(context.Background(), opts, Artifacts{XrayJSON: xrayJSON, MetaJSON: metaJSON})
	if err != nil {
		t.Fatalf("ApplyCandidate: %v", err)
	}
	if result != RuntimeApplyNoop {
		t.Fatalf("result = %s, want %s", result, RuntimeApplyNoop)
	}
	if _, exists, err := apply.ReadRequest(opts.RequestPath); err != nil || !exists {
		t.Fatalf("queued request was consumed: exists=%v err=%v", exists, err)
	}
}

func TestTryApplyRoutingPendingRejectsChangedSource(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	xrayJSON := []byte(`{"api":{"listen":"127.0.0.1:10085"}}`)
	metaJSON := []byte(`{"role":"client"}`)
	writeLive(t, opts.LiveDir, xrayJSON, metaJSON)
	desiredPath := filepath.Join(root, "xp2p-client.toml")
	if err := os.WriteFile(desiredPath, []byte("generation = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	opts.DesiredConfig = desiredPath
	opts.SourceDigest = apply.SourceDigest
	opts.Compile = func(_, _ string) (Artifacts, error) {
		if err := os.WriteFile(desiredPath, []byte("generation = 2\n"), 0o600); err != nil {
			return Artifacts{}, err
		}
		return Artifacts{XrayJSON: xrayJSON, MetaJSON: metaJSON}, nil
	}

	result, err := TryApplyRoutingPending(context.Background(), opts)
	if err != nil {
		t.Fatalf("TryApplyRoutingPending: %v", err)
	}
	if result != RuntimeApplySkipped {
		t.Fatalf("result = %s, want %s", result, RuntimeApplySkipped)
	}
	if _, exists, err := apply.ReadRequest(opts.RequestPath); err != nil || !exists {
		t.Fatalf("request was not retained: exists=%v err=%v", exists, err)
	}
}

func TestApplyCandidateRestoresLiveWhenDesiredCommitFails(t *testing.T) {
	root := t.TempDir()
	opts := testOptions(t, root, apply.RoleClient)
	xrayJSON := []byte(`{"api":{"listen":"127.0.0.1:10085"}}`)
	oldMeta := []byte(`{"role":"client","version":"old"}`)
	newMeta := []byte(`{"role":"client","version":"new"}`)
	writeLive(t, opts.LiveDir, xrayJSON, oldMeta)
	opts.CommitDesired = func() error { return errors.New("disk full") }

	result, err := ApplyCandidate(context.Background(), opts, Artifacts{XrayJSON: xrayJSON, MetaJSON: newMeta})
	if err != nil {
		t.Fatalf("ApplyCandidate: %v", err)
	}
	if result != RuntimeApplyFailed {
		t.Fatalf("result = %s, want %s", result, RuntimeApplyFailed)
	}
	got, err := os.ReadFile(filepath.Join(opts.LiveDir, layout.RuntimeMetaFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(oldMeta) {
		t.Fatalf("live metadata = %s, want restored %s", got, oldMeta)
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
	tags  []string
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
	return &testRoutingApplier{tags: append([]string(nil), tags...)}
}

func (a *testRoutingApplier) AddRule(_ context.Context, rule map[string]any) error {
	tag, _ := rule["ruleTag"].(string)
	a.calls = append(a.calls, "add:"+tag)
	a.tags = append(a.tags, tag)
	return nil
}

func (a *testRoutingApplier) RemoveRule(_ context.Context, tag string) error {
	a.calls = append(a.calls, "remove:"+tag)
	for index, current := range a.tags {
		if current == tag {
			a.tags = append(a.tags[:index], a.tags[index+1:]...)
			break
		}
	}
	return nil
}

func (a *testRoutingApplier) ListRuleTags(context.Context) ([]string, error) {
	return append([]string(nil), a.tags...), nil
}
