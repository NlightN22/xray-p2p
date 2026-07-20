package xraylive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/runtimeapply"
	"github.com/NlightN22/xray-p2p/go/internal/xrayapi"
)

type RuntimeApplyResult string

const (
	RuntimeApplySkipped              RuntimeApplyResult = "skipped"
	RuntimeApplyNoop                 RuntimeApplyResult = "noop"
	RuntimeApplyApplied              RuntimeApplyResult = "applied"
	RuntimeApplyStaged               RuntimeApplyResult = "staged"
	RuntimeApplyFailed               RuntimeApplyResult = "failed"
	RuntimeApplyUnsupported          RuntimeApplyResult = "unsupported"
	RuntimeApplyServiceLayerRequired RuntimeApplyResult = "service_layer_required"
)

type Artifacts struct {
	XrayJSON []byte
	MetaJSON []byte
	Extra    map[string][]byte
}

type CompileFunc func(configPath, extensionsDir string) (Artifacts, error)
type SourceDigestFunc func(configPath, extensionsDir string) (string, error)

type RoutingApplierFactory func(ctx context.Context, address string) (runtimeapply.RoutingApplier, func() error, error)
type InboundApplierFactory func(ctx context.Context, address string) (runtimeapply.InboundApplier, func() error, error)
type OutboundApplierFactory func(ctx context.Context, address string) (runtimeapply.OutboundApplier, func() error, error)
type InboundUserApplierFactory func(ctx context.Context, address string) (runtimeapply.InboundUserApplier, func() error, error)

type Options struct {
	Role           string
	RequestPath    string
	ErrorPath      string
	AuditPath      string
	DesiredConfig  string
	ExtensionsDir  string
	LiveDir        string
	LkgDir         string
	Compile        CompileFunc
	SourceDigest   SourceDigestFunc
	CommitDesired  func() error
	VerifyRuntime  func(context.Context) error
	NewApplier     RoutingApplierFactory
	NewInbound     InboundApplierFactory
	NewOutbound    OutboundApplierFactory
	NewInboundUser InboundUserApplierFactory
}

func ApplyCandidate(ctx context.Context, opts Options, artifacts Artifacts) (RuntimeApplyResult, error) {
	role := strings.TrimSpace(strings.ToLower(opts.Role))
	if role == "" {
		return RuntimeApplySkipped, errors.New("runtime apply role is required")
	}
	return applyRuntimeArtifacts(ctx, opts, apply.Request{Role: role}, role, artifacts)
}

func TryApplyRoutingPending(ctx context.Context, opts Options) (RuntimeApplyResult, error) {
	role := strings.TrimSpace(strings.ToLower(opts.Role))
	if role == "" {
		return RuntimeApplySkipped, errors.New("runtime apply role is required")
	}
	req, ok, err := readMatchingRequest(opts.RequestPath, opts.ErrorPath, role)
	if err != nil || !ok {
		return RuntimeApplySkipped, err
	}
	if opts.Compile == nil {
		return RuntimeApplySkipped, errors.New("runtime apply compile callback is required")
	}

	var sourceDigest string
	if opts.SourceDigest != nil {
		sourceDigest, err = opts.SourceDigest(opts.DesiredConfig, opts.ExtensionsDir)
		if err != nil {
			return RuntimeApplySkipped, err
		}
	}
	artifacts, err := opts.Compile(opts.DesiredConfig, opts.ExtensionsDir)
	if err != nil {
		writeRuntimeApplyError(opts, req, err)
		logging.Warn("runtime apply compilation failed", "role", role, "request_id", req.ID, "err", err)
		return RuntimeApplyFailed, nil
	}
	if opts.SourceDigest != nil {
		currentDigest, digestErr := opts.SourceDigest(opts.DesiredConfig, opts.ExtensionsDir)
		if digestErr != nil {
			return RuntimeApplySkipped, digestErr
		}
		if currentDigest != sourceDigest {
			logging.Info("desired config changed during compilation; runtime apply retained", "role", role, "request_id", req.ID)
			return RuntimeApplySkipped, nil
		}
	}
	return applyRuntimeArtifacts(ctx, opts, req, role, artifacts)
}

func applyRuntimeArtifacts(ctx context.Context, opts Options, req apply.Request, role string, artifacts Artifacts) (RuntimeApplyResult, error) {
	currentMeta, err := os.ReadFile(filepath.Join(opts.LiveDir, layout.RuntimeMetaFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RuntimeApplyServiceLayerRequired, nil
		}
		return RuntimeApplySkipped, fmt.Errorf("read live runtime metadata: %w", err)
	}
	serviceLayerRequired, err := runtimeMetaRequiresServiceLayer(currentMeta, artifacts.MetaJSON)
	if err != nil {
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	if serviceLayerRequired {
		logging.Info("runtime apply requires service layer", "role", role, "request_id", req.ID)
		return RuntimeApplyServiceLayerRequired, nil
	}

	current, err := os.ReadFile(filepath.Join(opts.LiveDir, layout.XrayConfigFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RuntimeApplyServiceLayerRequired, nil
		}
		return RuntimeApplySkipped, fmt.Errorf("read live xray config: %w", err)
	}
	normalized, err := preserveCurrentAPIListen(current, artifacts.XrayJSON)
	if err != nil {
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	if normalized != nil {
		artifacts.XrayJSON = normalized
	}
	diff, err := runtimeapply.ClassifyXrayConfigDiff(current, artifacts.XrayJSON)
	if err != nil {
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	switch diff.Kind {
	case runtimeapply.DiffNoop:
		if runtimeVerificationFailed(ctx, opts, req, nil) {
			return RuntimeApplyFailed, nil
		}
		if err := publishLiveArtifacts(opts, artifacts); err != nil {
			writeRuntimeApplyError(opts, req, err)
			return RuntimeApplyFailed, nil
		}
		if err := commitDesiredOrRestoreLive(opts); err != nil {
			writeRuntimeApplyError(opts, req, err)
			return RuntimeApplyFailed, nil
		}
		cleanupRuntimeApplyMarkers(opts, req)
		return RuntimeApplyNoop, nil
	case runtimeapply.DiffUnsupported:
		logging.Info("runtime apply unsupported", "role", role, "request_id", req.ID, "reason", diff.Reason)
		return RuntimeApplyUnsupported, nil
	case runtimeapply.DiffRoutingOnly, runtimeapply.DiffInboundOnly, runtimeapply.DiffOutboundOnly, runtimeapply.DiffInboundUsers, runtimeapply.DiffMixed:
	default:
		return RuntimeApplyUnsupported, nil
	}

	address, err := xrayapi.APIListenFromConfig(current)
	if err != nil || strings.TrimSpace(address) == "" {
		if err == nil {
			err = errors.New("xray API listen address is empty")
		}
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	switch diff.Kind {
	case runtimeapply.DiffRoutingOnly:
		return applyRoutingRuntimeDiff(ctx, opts, req, role, address, artifacts, diff)
	case runtimeapply.DiffInboundOnly:
		return applyInboundRuntimeDiff(ctx, opts, req, role, address, artifacts, diff)
	case runtimeapply.DiffOutboundOnly:
		return applyOutboundRuntimeDiff(ctx, opts, req, role, address, artifacts, diff)
	case runtimeapply.DiffInboundUsers:
		return applyInboundUserRuntimeDiff(ctx, opts, req, role, address, artifacts, diff)
	case runtimeapply.DiffMixed:
		return applyMixedRuntimeDiff(ctx, opts, req, role, address, artifacts, diff)
	default:
		return RuntimeApplyUnsupported, nil
	}
}

func applyRoutingRuntimeDiff(ctx context.Context, opts Options, req apply.Request, role, address string, artifacts Artifacts, diff runtimeapply.Diff) (RuntimeApplyResult, error) {
	factory := opts.NewApplier
	if factory == nil {
		factory = DefaultRoutingApplierFactory
	}
	applier, closeApplier, err := factory(ctx, address)
	if err != nil {
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	defer func() {
		if closeApplier != nil {
			_ = closeApplier()
		}
	}()
	if err := runtimeapply.ApplyRoutingDiff(ctx, applier, diff); err != nil {
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	if runtimeVerificationFailed(ctx, opts, req, func() error {
		return runtimeapply.ApplyRoutingDiff(ctx, applier, reverseRoutingDiff(diff))
	}) {
		return RuntimeApplyFailed, nil
	}
	if err := publishLiveArtifacts(opts, artifacts); err != nil {
		rollbackErr := runtimeapply.ApplyRoutingDiff(ctx, applier, reverseRoutingDiff(diff))
		reason := err
		if rollbackErr != nil {
			reason = fmt.Errorf("%w; runtime rollback failed: %v", err, rollbackErr)
		}
		writeRuntimeApplyError(opts, req, reason)
		return RuntimeApplyFailed, nil
	}
	if err := commitDesiredOrRestoreLive(opts); err != nil {
		rollbackErr := runtimeapply.ApplyRoutingDiff(ctx, applier, reverseRoutingDiff(diff))
		reason := err
		if rollbackErr != nil {
			reason = fmt.Errorf("%w; runtime rollback failed: %v", err, rollbackErr)
		}
		writeRuntimeApplyError(opts, req, reason)
		return RuntimeApplyFailed, nil
	}

	cleanupRuntimeApplyMarkers(opts, req)
	logging.Info("runtime routing apply completed", "role", role, "request_id", req.ID)
	return RuntimeApplyApplied, nil
}

func applyInboundRuntimeDiff(ctx context.Context, opts Options, req apply.Request, role, address string, artifacts Artifacts, diff runtimeapply.Diff) (RuntimeApplyResult, error) {
	factory := opts.NewInbound
	if factory == nil {
		factory = DefaultInboundApplierFactory
	}
	applier, closeApplier, err := factory(ctx, address)
	if err != nil {
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	defer func() {
		if closeApplier != nil {
			_ = closeApplier()
		}
	}()
	if err := runtimeapply.ApplyInboundDiff(ctx, applier, diff); err != nil {
		writeRuntimeApplyError(opts, req, err)
		return RuntimeApplyFailed, nil
	}
	if runtimeVerificationFailed(ctx, opts, req, func() error {
		return runtimeapply.ApplyInboundDiff(ctx, applier, reverseInboundDiff(diff))
	}) {
		return RuntimeApplyFailed, nil
	}
	if err := publishLiveArtifacts(opts, artifacts); err != nil {
		rollbackErr := runtimeapply.ApplyInboundDiff(ctx, applier, reverseInboundDiff(diff))
		reason := err
		if rollbackErr != nil {
			reason = fmt.Errorf("%w; runtime rollback failed: %v", err, rollbackErr)
		}
		writeRuntimeApplyError(opts, req, reason)
		return RuntimeApplyFailed, nil
	}
	if err := commitDesiredOrRestoreLive(opts); err != nil {
		rollbackErr := runtimeapply.ApplyInboundDiff(ctx, applier, reverseInboundDiff(diff))
		reason := err
		if rollbackErr != nil {
			reason = fmt.Errorf("%w; runtime rollback failed: %v", err, rollbackErr)
		}
		writeRuntimeApplyError(opts, req, reason)
		return RuntimeApplyFailed, nil
	}

	cleanupRuntimeApplyMarkers(opts, req)
	logging.Info("runtime inbound apply completed", "role", role, "request_id", req.ID)
	return RuntimeApplyApplied, nil
}

func runtimeVerificationFailed(ctx context.Context, opts Options, req apply.Request, rollback func() error) bool {
	if opts.VerifyRuntime == nil {
		return false
	}
	if err := opts.VerifyRuntime(ctx); err != nil {
		reason := fmt.Errorf("runtime verification failed: %w", err)
		if rollback != nil {
			if rollbackErr := rollback(); rollbackErr != nil {
				reason = fmt.Errorf("%w; runtime rollback failed: %v", reason, rollbackErr)
			}
		}
		writeRuntimeApplyError(opts, req, reason)
		return true
	}
	return false
}

func DefaultRoutingApplierFactory(ctx context.Context, address string) (runtimeapply.RoutingApplier, func() error, error) {
	client, err := xrayapi.Dial(ctx, address, xrayapi.DefaultTimeout)
	if err != nil {
		return nil, nil, err
	}
	return client, client.Close, nil
}

func DefaultInboundApplierFactory(ctx context.Context, address string) (runtimeapply.InboundApplier, func() error, error) {
	client, err := xrayapi.Dial(ctx, address, xrayapi.DefaultTimeout)
	if err != nil {
		return nil, nil, err
	}
	return xrayInboundApplier{client: client}, client.Close, nil
}

type xrayInboundApplier struct {
	client *xrayapi.Client
}

func (a xrayInboundApplier) AddInbound(ctx context.Context, inbound map[string]any) error {
	cfg, err := xrayapi.InboundFromMap(inbound)
	if err != nil {
		return err
	}
	return a.client.AddInbound(ctx, cfg)
}

func (a xrayInboundApplier) RemoveInbound(ctx context.Context, tag string) error {
	return a.client.RemoveInbound(ctx, tag)
}

func (a xrayInboundApplier) ListInboundTags(ctx context.Context) ([]string, error) {
	return a.client.ListInboundTags(ctx)
}

func readMatchingRequest(requestPath, errorPath, role string) (apply.Request, bool, error) {
	req, exists, err := apply.ReadRequestForRole(requestPath, role)
	if err != nil || !exists {
		return req, false, err
	}
	if marker, markerExists, err := apply.ReadError(errorPath); err != nil {
		return req, false, err
	} else if markerExists && marker.RequestID != "" && marker.RequestID == req.ID {
		logging.Warn("runtime apply skipped (previous failure)", "role", role, "request_id", req.ID, "reason", marker.Reason)
		return req, false, nil
	}
	return req, true, nil
}

func publishLiveArtifacts(opts Options, artifacts Artifacts) error {
	files := map[string][]byte{
		layout.XrayConfigFileName:  artifacts.XrayJSON,
		layout.RuntimeMetaFileName: artifacts.MetaJSON,
	}
	for name, data := range artifacts.Extra {
		files[name] = data
	}
	return apply.ReplaceRoleLiveDir(opts.LiveDir, opts.LkgDir, files)
}

func commitDesiredOrRestoreLive(opts Options) error {
	if opts.CommitDesired == nil {
		return nil
	}
	if err := opts.CommitDesired(); err != nil {
		restoreErr := apply.NewRollback(opts.LiveDir, opts.LkgDir).Restore(opts.AuditPath)
		if restoreErr != nil {
			return fmt.Errorf("commit Desired: %w; restore Live failed: %v", err, restoreErr)
		}
		return fmt.Errorf("commit Desired: %w", err)
	}
	return nil
}

func cleanupRuntimeApplyMarkers(opts Options, req apply.Request) {
	if strings.TrimSpace(opts.RequestPath) == "" || strings.TrimSpace(req.ID) == "" {
		return
	}
	if err := apply.CompleteRequest(opts.RequestPath, opts.ErrorPath, req); err != nil {
		logging.Warn("runtime apply marker cleanup failed", "role", req.Role, "request_id", req.ID, "err", err)
	}
}

func writeRuntimeApplyError(opts Options, req apply.Request, err error) {
	if err == nil {
		return
	}
	if strings.TrimSpace(opts.ErrorPath) == "" || strings.TrimSpace(req.ID) == "" {
		logging.Warn("runtime apply failed", "role", opts.Role, "err", err)
		return
	}
	_ = apply.WriteError(opts.ErrorPath, apply.ErrorMarker{
		RequestID: req.ID,
		Role:      opts.Role,
		Reason:    err.Error(),
	}, opts.AuditPath)
	logging.Warn("runtime apply failed", "role", opts.Role, "request_id", req.ID, "err", err)
}

func preserveCurrentAPIListen(current, candidate []byte) ([]byte, error) {
	currentListen, err := xrayapi.APIListenFromConfig(current)
	if err != nil || strings.TrimSpace(currentListen) == "" {
		return nil, err
	}
	candidateListen, err := xrayapi.APIListenFromConfig(candidate)
	if err != nil || strings.TrimSpace(candidateListen) == "" {
		return nil, err
	}
	if strings.TrimSpace(candidateListen) == strings.TrimSpace(currentListen) {
		return nil, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(candidate, &doc); err != nil {
		return nil, fmt.Errorf("parse candidate xray config: %w", err)
	}
	apiDoc, ok := doc["api"].(map[string]any)
	if !ok {
		return nil, errors.New("candidate xray config api section is missing")
	}
	apiDoc["listen"] = strings.TrimSpace(currentListen)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode candidate xray config: %w", err)
	}
	return append(data, '\n'), nil
}

func runtimeMetaRequiresServiceLayer(current, candidate []byte) (bool, error) {
	currentDoc, err := parseRuntimeMeta(current, "current")
	if err != nil {
		return false, err
	}
	candidateDoc, err := parseRuntimeMeta(candidate, "candidate")
	if err != nil {
		return false, err
	}
	for _, key := range serviceLayerRuntimeMetaKeys() {
		currentValue, currentSet := currentDoc[key]
		candidateValue, candidateSet := candidateDoc[key]
		if !currentSet && !candidateSet {
			continue
		}
		if !currentSet || !candidateSet || !reflect.DeepEqual(currentValue, candidateValue) {
			return true, nil
		}
	}
	return false, nil
}

func parseRuntimeMeta(data []byte, label string) (map[string]any, error) {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s runtime metadata: %w", label, err)
	}
	return doc, nil
}

func serviceLayerRuntimeMetaKeys() []string {
	return []string{
		"tun_enabled",
		"tun_name",
		"tun_mtu",
		"tun_addr",
		"tun_mode",
		"dns_servers",
		"full_tunnel_tag",
	}
}

func reverseRoutingDiff(diff runtimeapply.Diff) runtimeapply.Diff {
	reversed := runtimeapply.Diff{Kind: runtimeapply.DiffRoutingOnly}
	for _, change := range diff.AddedRules {
		reversed.RemovedRuleTag = append(reversed.RemovedRuleTag, change.RuleTag)
		reversed.RemovedRules = append(reversed.RemovedRules, change)
	}
	for _, change := range diff.RemovedRules {
		reversed.AddedRules = append(reversed.AddedRules, change)
	}
	return reversed
}

func reverseInboundDiff(diff runtimeapply.Diff) runtimeapply.Diff {
	reversed := runtimeapply.Diff{Kind: runtimeapply.DiffInboundOnly}
	for _, change := range diff.AddedInbounds {
		reversed.RemovedInboundTags = append(reversed.RemovedInboundTags, change.Tag)
		reversed.RemovedInbounds = append(reversed.RemovedInbounds, change)
	}
	for _, change := range diff.RemovedInbounds {
		reversed.AddedInbounds = append(reversed.AddedInbounds, change)
	}
	return reversed
}
