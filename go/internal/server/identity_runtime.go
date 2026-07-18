//go:build windows || linux

package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
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
		artifacts, err := compileServerRuntimeCandidateDoc(doc)
		if err != nil {
			return xraylive.RuntimeApplySkipped, err
		}
		if err := stampIdentityTransactionCandidate(doc, artifacts); err != nil {
			return xraylive.RuntimeApplySkipped, err
		}
		result, err := commitServerRuntimeDocResult(ctx, doc)
		if stampErr := stampIdentityTransactionRuntimeResult(result); stampErr != nil && err == nil {
			return result, stampErr
		}
		return result, err
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
	artifacts := xraylive.Artifacts{
		XrayJSON: compiled.XrayJSON,
		MetaJSON: compiled.MetaJSON,
		Extra:    extra,
	}
	if err := stampIdentityTransactionCandidate(doc, artifacts); err != nil {
		return xraylive.RuntimeApplySkipped, err
	}
	result, err := applyServerRuntimeCandidate(ctx, artifacts)
	if stampErr := stampIdentityTransactionRuntimeResult(result); stampErr != nil && err == nil {
		return result, stampErr
	}
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

func stampIdentityTransactionCandidate(doc map[string]any, artifacts xraylive.Artifacts) error {
	desiredHash, err := identitysync.HashJSON(doc)
	if err != nil {
		return err
	}
	candidateLiveHash := hashRuntimeArtifacts(artifacts)
	previousDesiredHash := hashServerStateDocIfExists(pendingConfigPath())
	previousLiveHash := hashLiveArtifactsIfExists()
	return identitysync.DefaultStore().UpdateTransaction(func(tx *identitysync.Transaction) error {
		tx.PreviousDesiredHash = previousDesiredHash
		tx.CandidateDesiredHash = desiredHash
		tx.PreviousLiveHash = previousLiveHash
		tx.CandidateLiveHash = candidateLiveHash
		return nil
	})
}

func stampIdentityTransactionRuntimeResult(result xraylive.RuntimeApplyResult) error {
	return identitysync.DefaultStore().UpdateTransaction(func(tx *identitysync.Transaction) error {
		tx.RuntimeResult = string(result)
		return nil
	})
}

func hashRuntimeArtifacts(artifacts xraylive.Artifacts) string {
	data := make([]byte, 0, len(artifacts.XrayJSON)+len(artifacts.MetaJSON))
	data = append(data, artifacts.XrayJSON...)
	data = append(data, '\n')
	data = append(data, artifacts.MetaJSON...)
	for _, name := range sortedArtifactNames(artifacts.Extra) {
		data = append(data, '\n')
		data = append(data, []byte(name)...)
		data = append(data, 0)
		data = append(data, artifacts.Extra[name]...)
	}
	return identitysync.HashBytes(data)
}

func sortedArtifactNames(values map[string][]byte) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func hashLiveArtifactsIfExists() string {
	liveDir, err := config.LiveRoleDir(apply.RoleServer)
	if err != nil {
		return ""
	}
	xrayJSON, err := os.ReadFile(filepath.Join(liveDir, layout.XrayConfigFileName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ""
	}
	metaJSON, err := os.ReadFile(filepath.Join(liveDir, layout.RuntimeMetaFileName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if len(xrayJSON) == 0 && len(metaJSON) == 0 {
		return ""
	}
	extra := map[string][]byte{}
	entries, err := os.ReadDir(liveDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := filepath.Clean(entry.Name())
			if name == layout.XrayConfigFileName || name == layout.RuntimeMetaFileName {
				continue
			}
			data, readErr := os.ReadFile(filepath.Join(liveDir, entry.Name()))
			if readErr == nil {
				extra[name] = data
			}
		}
	}
	return hashRuntimeArtifacts(xraylive.Artifacts{XrayJSON: xrayJSON, MetaJSON: metaJSON, Extra: extra})
}

func hashServerStateDocIfExists(path string) string {
	doc, err := loadServerStateDoc(path)
	if err != nil {
		return ""
	}
	hash, err := identitysync.HashJSON(doc)
	if err != nil {
		return ""
	}
	return hash
}
