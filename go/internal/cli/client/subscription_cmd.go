package clientcmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/client"
)

type subscriptionStatusResult struct {
	Subscriptions []subscriptionStatusItem `json:"subscriptions"`
}

type subscriptionStatusItem struct {
	ID              string  `json:"id"`
	Adapter         string  `json:"adapter"`
	Revision        string  `json:"revision"`
	OfferCount      int     `json:"offer_count"`
	SelectedOfferID *string `json:"selected_offer_id"`
	LastRefreshAt   *string `json:"last_refresh_at"`
	LastApplyAt     *string `json:"last_apply_at"`
	LastError       *string `json:"last_error"`
}

type subscriptionOffersResult struct {
	Offers []subscriptionOfferItem `json:"offers"`
}

type subscriptionOfferItem struct {
	SubscriptionID string `json:"subscription_id"`
	StableID       string `json:"stable_id"`
	Protocol       string `json:"protocol"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	UserLabel      string `json:"user_label"`
}

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
		&cobra.Command{Use: "status", Short: "Show external subscription status", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			statuses, err := client.ListExternalSubscriptions()
			if err != nil {
				return err
			}
			if clioutput.Enabled(cmd) {
				result := subscriptionStatusResult{Subscriptions: make([]subscriptionStatusItem, 0, len(statuses))}
				for _, status := range statuses {
					result.Subscriptions = append(result.Subscriptions, subscriptionStatusItem{
						ID: status.ID, Adapter: status.Adapter, Revision: status.Revision,
						OfferCount: len(status.Offers), SelectedOfferID: optionalString(status.SelectedOfferID),
						LastRefreshAt: optionalTime(status.LastRefreshAt), LastApplyAt: optionalTime(status.LastApplyAt),
						LastError: optionalString(status.LastError),
					})
				}
				return clioutput.SetResult(cmd, result)
			}
			if len(statuses) == 0 {
				fmt.Println("No external subscriptions configured.")
				return nil
			}
			for _, status := range statuses {
				fmt.Printf("ID: %s\nAdapter: %s\nRevision: %s\nOffers: %d\nSelected offer: %s\nLast refresh: %s\nLast apply: %s\n", status.ID, status.Adapter, status.Revision, len(status.Offers), formatSelectedOffer(status.SelectedOfferID), formatSubscriptionTime(status.LastRefreshAt), formatSubscriptionTime(status.LastApplyAt))
				if status.LastError != "" {
					fmt.Printf("Last error: %s\n", status.LastError)
				}
			}
			return nil
		}},
		&cobra.Command{Use: "offers", Short: "List external connection offers", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			statuses, err := client.ListExternalSubscriptions()
			if err != nil {
				return err
			}
			if clioutput.Enabled(cmd) {
				result := subscriptionOffersResult{Offers: make([]subscriptionOfferItem, 0)}
				for _, status := range statuses {
					for _, offer := range status.Offers {
						result.Offers = append(result.Offers, subscriptionOfferItem{
							SubscriptionID: status.ID, StableID: offer.StableID,
							Protocol: offer.Endpoint.Protocol, Host: offer.Endpoint.Host,
							Port: offer.Endpoint.Port, UserLabel: offer.UserLabel,
						})
					}
				}
				return clioutput.SetResult(cmd, result)
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

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func optionalTime(value time.Time) *string {
	if value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func formatSelectedOffer(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
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
