package clientcmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/spf13/cobra"
)

func newClientEndpointGroupCmd(_ commandConfig) *cobra.Command {
	cmd := &cobra.Command{Use: "group", Short: "Inspect HA endpoint groups"}
	cmd.AddCommand(&cobra.Command{
		Use: "list", Aliases: []string{"status", "inspect"}, Short: "List HA endpoint groups",
		RunE: func(_ *cobra.Command, _ []string) error {
			records, err := client.ListEndpointGroups()
			if err != nil {
				return err
			}
			writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "GROUP ID\tTAG\tMODE\tACTIVE\tMEMBERS\tCOOLDOWN\tREVISION")
			for _, record := range records {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%d\n", record.GroupID, record.Tag, record.Mode, record.ActiveTag, strings.Join(record.Members, ","), record.CooldownUntil, record.Revision)
			}
			return writer.Flush()
		},
	})
	return cmd
}
