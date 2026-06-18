//go:build linux || windows

package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
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
	if err := state.save(configFile); err != nil {
		return err
	}
	return writeClientApplyRequestAndTryRuntime(ctx)
}

func writeClientApplyRequestAndTryRuntime(ctx context.Context) error {
	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return err
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		return err
	}
	_, err = tryRuntimeApplyPending(ctx, apply.RoleClient)
	return err
}
