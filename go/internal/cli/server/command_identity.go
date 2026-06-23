package servercmd

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/identitysync"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/spf13/cobra"
)

func newServerIdentityCmd(cfg commandConfig) *cobra.Command {
	cmd := &cobra.Command{Use: "identity", Short: "Manage identity cache operations"}
	cmd.AddCommand(
		newServerIdentitySyncCmd(cfg),
		newServerIdentityStatusCmd(),
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
		status, err := service.Sync(commandContext(cmd), provider)
		if err != nil {
			return err
		}
		fmt.Printf("identity sync: %s\n", status.State)
		return nil
	}}
}

func newServerIdentityStatusCmd() *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Show identity cache status", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		state, err := identitysync.DefaultStore().Load()
		if err != nil {
			return err
		}
		printIdentityState(state)
		return nil
	}}
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

func printIdentityState(state identitysync.State) {
	fmt.Printf("status: %s\n", state.Status.State)
	if state.Status.LastSuccess != "" {
		fmt.Printf("last_success: %s\n", state.Status.LastSuccess)
	}
	if state.Status.Error != "" {
		fmt.Printf("error: %s\n", state.Status.Error)
	}
	if state.Provider != nil {
		fmt.Printf("provider: %s (%s)\n", state.Provider.InstanceID, state.Provider.Kind)
	}
	if state.Current == nil {
		fmt.Println("current: none")
		return
	}
	detached := ""
	if state.Current.Detached {
		detached = " detached"
	}
	fmt.Printf("generation: %s%s\n", state.Current.ID, detached)
	for _, subject := range state.Current.Subjects {
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
		fmt.Printf("user %s %s groups=%s%s\n", subject.UserLabel, subject.ExternalSubject, strings.Join(subject.DirectGroups, ","), suffix)
	}
	for _, group := range state.Current.Groups {
		fmt.Printf("group %s members=%s groups=%s\n", group.ID, strings.Join(group.DirectMembers, ","), strings.Join(group.DirectGroups, ","))
	}
}
