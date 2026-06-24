//go:build linux || windows

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

// UpdateEndpointOptions controls a credentials-only endpoint update.
type UpdateEndpointOptions struct {
	InstallDir  string
	ConfigDir   string
	Target      string
	User        string
	Password    string
	UserSet     bool
	PasswordSet bool
}

// UpdateEndpointCredentials updates only the selected endpoint user/password fields.
func UpdateEndpointCredentials(ctx context.Context, opts UpdateEndpointOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target := strings.TrimSpace(opts.Target)
	if target == "" {
		return errors.New("endpoint hostname or tag is required")
	}
	if !opts.UserSet && !opts.PasswordSet {
		return errors.New("at least one of user or password is required")
	}
	if opts.UserSet && strings.TrimSpace(opts.User) == "" {
		return errors.New("user is required")
	}
	if opts.PasswordSet && strings.TrimSpace(opts.Password) == "" {
		return errors.New("password is required")
	}

	configFile := config.ConfigPath(layout.ClientConfigFileName)
	state, err := loadClientInstallState(configFile)
	if err != nil {
		return err
	}
	if _, ok := state.updateEndpointCredentials(target, opts.User, opts.Password, opts.UserSet, opts.PasswordSet); !ok {
		return fmt.Errorf("client endpoint %q not found", target)
	}
	return commitClientRuntimeState(ctx, state)
}

func compileClientRuntimeCandidate(state clientInstallState) (xraylive.Artifacts, error) {
	return compileClientRuntimeCandidateWithSelector(state, nil)
}

func compileClientRuntimeCandidateWithSelector(state clientInstallState, selector *endpointSelectorState) (xraylive.Artifacts, error) {
	sourcePath := config.ConfigPath(layout.ClientConfigFileName)
	file, err := os.CreateTemp("", "xp2p-client-candidate-*.toml")
	if err != nil {
		return xraylive.Artifacts{}, err
	}
	candidatePath := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(candidatePath)
		return xraylive.Artifacts{}, err
	}
	defer os.Remove(candidatePath)
	if data, err := os.ReadFile(sourcePath); err == nil {
		if err := os.WriteFile(candidatePath, data, 0o644); err != nil {
			return xraylive.Artifacts{}, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return xraylive.Artifacts{}, err
	}
	if err := state.save(candidatePath); err != nil {
		return xraylive.Artifacts{}, err
	}
	extensionsDir, err := config.DesiredExtensionsDirForRole("client")
	if err != nil {
		return xraylive.Artifacts{}, err
	}
	artifacts, err := compileDesiredWithSelector(candidatePath, extensionsDir, selector)
	if err != nil {
		return xraylive.Artifacts{}, err
	}
	result := xraylive.Artifacts{XrayJSON: artifacts.XrayJSON, MetaJSON: artifacts.MetaJSON}
	if selector != nil {
		data, err := json.MarshalIndent(selector, "", "  ")
		if err != nil {
			return xraylive.Artifacts{}, err
		}
		result.Extra = map[string][]byte{
			layout.ClientEndpointSelectorStateFileName:   append(data, '\n'),
			layout.ClientEndpointSelectorJournalFileName: append(data, '\n'),
		}
	}
	return result, nil
}
