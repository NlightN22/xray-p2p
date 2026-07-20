package clientcmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/client"
)

func newClientSubscriptionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "subscription", Short: "Manage external server-authoritative subscriptions"}
	var addAllowHTTP bool
	add := &cobra.Command{Use: "add <id> <url>", Short: "Add and fetch an external subscription", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return client.AddExternalSubscription(commandContext(cmd), client.ExternalSubscriptionOptions{ID: args[0], URL: args[1], AllowHTTP: addAllowHTTP})
	}}
	add.Flags().BoolVarP(&addAllowHTTP, "allow-http", "A", false, "allow HTTP for a local compatibility fixture")
	var refreshAllowHTTP bool
	refresh := &cobra.Command{Use: "refresh <id>", Short: "Refresh one external subscription", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return client.RefreshExternalSubscription(commandContext(cmd), client.ExternalSubscriptionOptions{ID: args[0], AllowHTTP: refreshAllowHTTP})
	}}
	refresh.Flags().BoolVarP(&refreshAllowHTTP, "allow-http", "A", false, "allow HTTP for a local compatibility fixture")
	cmd.AddCommand(
		add,
		&cobra.Command{Use: "status", Short: "Show external subscription status", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
			statuses, err := client.ListExternalSubscriptions()
			if err != nil {
				return err
			}
			if len(statuses) == 0 {
				fmt.Println("No external subscriptions configured.")
				return nil
			}
			for _, status := range statuses {
				fmt.Printf("ID: %s\nAdapter: %s\nRevision: %s\nOffers: %d\nLast refresh: %s\nLast apply: %s\n", status.ID, status.Adapter, status.Revision, len(status.Offers), formatSubscriptionTime(status.LastRefreshAt), formatSubscriptionTime(status.LastApplyAt))
				if status.LastError != "" {
					fmt.Printf("Last error: %s\n", status.LastError)
				}
			}
			return nil
		}},
		&cobra.Command{Use: "offers", Short: "List external connection offers", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
			statuses, err := client.ListExternalSubscriptions()
			if err != nil {
				return err
			}
			for _, status := range statuses {
				for _, offer := range status.Offers {
					fmt.Printf("%s\t%s\t%s\t%s:%d\t%s\n", status.ID, offer.StableID, offer.Endpoint.Protocol, offer.Endpoint.Host, offer.Endpoint.Port, offer.UserLabel)
				}
			}
			return nil
		}},
		refresh,
		&cobra.Command{Use: "remove <id>", Short: "Remove one external subscription and its offers", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			return client.RemoveExternalSubscription(commandContext(cmd), args[0])
		}},
	)
	return cmd
}

func formatSubscriptionTime(value interface {
	IsZero() bool
	String() string
}) string {
	if value.IsZero() {
		return "-"
	}
	return strings.TrimSpace(value.String())
}
