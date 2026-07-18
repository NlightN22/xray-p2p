//go:build windows || linux

package server

import (
	"context"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func ApplyIdentityRuntime(ctx context.Context) (xraylive.RuntimeApplyResult, error) {
	state, err := identitysync.DefaultStore().Load()
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	doc, err := loadServerStateDoc(pendingConfigPath())
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	changed, err := reconcileAuthoritativeIdentityRemovals(doc, state)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	if changed {
		return commitServerRuntimeDocResult(ctx, doc)
	}

	extensionsDir, err := config.DesiredExtensionsDirForRole(apply.RoleServer)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	compiled, err := compileDesired(pendingConfigPath(), extensionsDir)
	if err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	extra := make(map[string][]byte, len(compiled.Extra))
	for name, data := range compiled.Extra {
		extra[filepath.Clean(name)] = data
	}
	result, err := applyServerRuntimeCandidate(ctx, xraylive.Artifacts{
		XrayJSON: compiled.XrayJSON,
		MetaJSON: compiled.MetaJSON,
		Extra:    extra,
	})
	if err != nil {
		return result, err
	}
	if result == xraylive.RuntimeApplyStaged || result == xraylive.RuntimeApplyServiceLayerRequired || result == xraylive.RuntimeApplyUnsupported {
		if reqErr := writeServerRuntimeApplyRequest(); reqErr != nil {
			return result, reqErr
		}
		return xraylive.RuntimeApplyStaged, nil
	}
	if result != xraylive.RuntimeApplyApplied && result != xraylive.RuntimeApplyNoop {
		return result, xraylive.ResultError(result)
	}
	return result, nil
}
