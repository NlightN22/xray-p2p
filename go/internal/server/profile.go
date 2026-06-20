//go:build windows || linux

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

type SetProfileOptions struct {
	Profile string
}

type SetProfileResult struct {
	Profile string
	Apply   xraylive.RuntimeApplyResult
}

func SetProfile(ctx context.Context, opts SetProfileOptions) (SetProfileResult, error) {
	profile, err := normalizeServerProfile(opts.Profile)
	if err != nil {
		return SetProfileResult{}, err
	}
	doc, err := loadServerStateDoc(pendingConfigPath())
	if err != nil {
		return SetProfileResult{}, err
	}
	doc["profile"] = profile
	result, err := commitServerRuntimeDocResult(ctx, doc)
	if err != nil {
		return SetProfileResult{}, err
	}
	return SetProfileResult{Profile: profile, Apply: result}, nil
}

func normalizeServerProfile(value string) (string, error) {
	endpoint, err := tunnel.DefaultProfile(tunnel.Profile(strings.TrimSpace(value)))
	if err != nil {
		return "", fmt.Errorf("invalid server profile: %w", err)
	}
	return string(endpoint.Profile), nil
}
