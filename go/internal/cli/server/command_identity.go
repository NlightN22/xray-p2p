package servercmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/usecase"
	"github.com/spf13/cobra"
)

func newServerIdentityCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{Use: "identity", Short: "Manage identity cache operations"}
	cmd.AddCommand(
		newServerIdentitySyncCmd(cfg),
		newServerIdentityStatusCmd(cfg),
		newServerIdentityProvisionCmd(cfg),
		newServerIdentityDetachCmd(),
		newServerIdentitySelectCmd(),
	)
	return cmd
}

func newServerIdentitySyncCmd(cfg commandConfig) *cobra.Command {
	return &cobra.Command{Use: "sync", Short: "Synchronize identity cache", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		provider, err := providerFromConfig(cfg())
		if err != nil {
			return err
		}
		service := identitysync.Service{
			Store:    identitysync.DefaultStore(),
			Fetcher:  identitysync.ConfigFetcher{Config: cfg().Server.IdentityProvider},
			Allocate: nil,
		}
		status, applyResult, err := service.SyncAndApply(commandContext(cmd), provider, func(ctx context.Context) (string, error) {
			result, err := server.ApplyIdentityRuntime(ctx)
			return string(result), err
		})
		if err != nil {
			return err
		}
		fmt.Printf("identity sync: %s\n", status.State)
		fmt.Printf("identity apply: %s\n", applyResult)
		return nil
	}}
}

func newServerIdentityStatusCmd(cfg commandConfig) *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Show identity cache status", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		view, err := usecase.NewIdentityStatus(identitysync.DefaultStore()).
			WithRedirects(serverIdentityRedirectLister{cfg: cfg()}).
			View(commandContext(cmd))
		if err != nil {
			return err
		}
		if clioutput.Enabled(cmd) {
			return clioutput.SetResult(cmd, view)
		}
		printIdentityState(view)
		return nil
	}}
}

type serverIdentityRedirectLister struct {
	cfg config.Config
}

func (l serverIdentityRedirectLister) ListIdentityRedirects(ctx context.Context) ([]usecase.IdentityRedirectView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	records, err := server.ListRedirects(server.RedirectListOptions{
		InstallDir: l.cfg.Server.InstallDir,
		ConfigDir:  l.cfg.Server.ConfigDir,
	})
	if err != nil {
		if errors.Is(err, server.ErrUnsupported) {
			return []usecase.IdentityRedirectView{}, nil
		}
		return nil, err
	}
	out := make([]usecase.IdentityRedirectView, 0, len(records))
	for _, record := range records {
		state := "enabled"
		if record.Disabled {
			state = "disabled"
		} else if record.DisabledByPolicy {
			state = "disabled_by_policy"
		}
		out = append(out, usecase.IdentityRedirectView{
			Type:        record.Type,
			Value:       record.Value,
			OutboundTag: record.Tag,
			Host:        record.Hostname,
			State:       state,
		})
	}
	return out, nil
}

func newServerIdentityProvisionCmd(cfg commandConfig) *cobra.Command {
	var host string
	cmd := &cobra.Command{Use: "provision <label>", Short: "Provision a cached identity as a server user", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		link, err := server.ProvisionIdentity(commandContext(cmd), server.ProvisionIdentityOptions{
			InstallDir: cfg().Server.InstallDir,
			ConfigDir:  cfg().Server.ConfigDir,
			Host:       firstNonEmpty(host, cfg().Server.Host),
			UserLabel:  args[0],
		})
		if err != nil {
			return err
		}
		if clioutput.Enabled(cmd) {
			return clioutput.SetResult(cmd, struct {
				UserID string `json:"user_id"`
				Link   string `json:"link"`
			}{UserID: args[0], Link: link.Link})
		}
		fmt.Println(link.Link)
		return nil
	}}
	cmd.Flags().StringVarP(&host, "host", "H", "", "public host name or IP for generated connection link")
	return cmd
}

func newServerIdentityDetachCmd() *cobra.Command {
	return &cobra.Command{Use: "detach", Short: "Detach the selected identity provider", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		if err := identitysync.DefaultStore().DetachProvider(); err != nil {
			return err
		}
		fmt.Println("identity provider detached")
		return nil
	}}
}

func newServerIdentitySelectCmd() *cobra.Command {
	var kind string
	var groups []string
	cmd := &cobra.Command{Use: "select <instance-id>", Short: "Select or reattach an identity provider", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		provider := identitysync.ProviderRef{InstanceID: args[0], Kind: identitysync.ProviderKind(kind), Scope: groups}
		if err := identitysync.DefaultStore().SelectProvider(provider); err != nil {
			return err
		}
		fmt.Println("identity provider selected")
		return nil
	}}
	cmd.Flags().StringVarP(&kind, "kind", "K", "", "provider kind: ldap or scim")
	cmd.Flags().StringSliceVarP(&groups, "group", "G", nil, "provider group scope")
	_ = cmd.MarkFlagRequired("kind")
	return cmd
}

func providerFromConfig(cfgv config.Config) (identitysync.ProviderRef, error) {
	raw := cfgv.Server.IdentityProvider
	provider := identitysync.ProviderRef{
		InstanceID: raw.InstanceID,
		Kind:       identitysync.ProviderKind(raw.Kind),
		Scope:      raw.GroupIDs,
	}
	return provider, provider.Validate()
}

func printIdentityState(view usecase.IdentityStatusView) {
	fmt.Printf("status: %s\n", view.Status)
	if view.LastSuccess != "" {
		fmt.Printf("last_success: %s\n", view.LastSuccess)
	}
	if view.Error != "" {
		fmt.Printf("error: %s\n", view.Error)
	}
	if view.ProviderID != "" {
		fmt.Printf("provider: %s (%s)\n", view.ProviderID, view.ProviderKind)
	}
	if view.Generation == "" {
		fmt.Println("current: none")
		return
	}
	detached := ""
	if view.Detached {
		detached = " detached"
	}
	fmt.Printf("generation: %s%s\n", view.Generation, detached)
	for _, subject := range view.Subjects {
		flags := []string{}
		if subject.Provisioned {
			flags = append(flags, "provisioned")
		}
		if !subject.Active {
			flags = append(flags, "inactive")
		}
		suffix := ""
		if len(flags) > 0 {
			suffix = " [" + strings.Join(flags, ",") + "]"
		}
		fmt.Printf("user %s %s groups=%s%s\n", subject.Label, subject.ExternalID, strings.Join(subject.DirectGroups, ","), suffix)
	}
	for _, group := range view.Groups {
		fmt.Printf("group %s members=%s groups=%s transitive_members=%s\n", group.ID, strings.Join(group.DirectMembers, ","), strings.Join(group.DirectGroups, ","), strings.Join(group.TransitiveMembers, ","))
	}
	for _, redirect := range view.Redirects {
		fmt.Printf("redirect %s %s tag=%s host=%s state=%s\n", redirect.Type, redirect.Value, redirect.OutboundTag, redirect.Host, redirect.State)
	}
}
