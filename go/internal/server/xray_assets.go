package server

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/xrayassets"
)

func syncXrayAssets(ctx context.Context, meta runtimeMeta, xrayPath, configDir string) error {
	cfg, err := xrayassets.FromConfig(meta.XrayAssets)
	if err != nil {
		return fmt.Errorf("xray asset preflight failed: %w", err)
	}
	return xrayassets.Sync(ctx, cfg, xrayassets.Options{
		AssetDir:       xrayassets.AssetDirForXrayPath(xrayPath),
		Role:           "server",
		XrayConfigPath: filepath.Join(configDir, layout.XrayConfigFileName),
	})
}
