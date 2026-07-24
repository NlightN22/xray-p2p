package clientcmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/spf13/cobra"
)

func newClientEndpointGroupCmd(_ commandConfig) *cobra.Command {
	cmd := &cobra.Command{Use: "group", Short: "Inspect HA endpoint groups"}
	cmd.AddCommand(&cobra.Command{
		Use: "list", Aliases: []string{"status", "inspect"}, Short: "List HA endpoint groups",
		RunE: func(cmd *cobra.Command, _ []string) error {
			records, err := client.ListEndpointGroups()
			if err != nil {
				return err
			}
			if clioutput.Enabled(cmd) {
				type groupResult struct {
					GroupID       string   `json:"group_id"`
					Tag           string   `json:"tag"`
					Mode          string   `json:"mode"`
					ActiveTag     string   `json:"active_tag"`
					Members       []string `json:"members"`
					CooldownUntil string   `json:"cooldown_until"`
					Revision      uint64   `json:"revision"`
				}
				result := struct {
					Groups []groupResult `json:"groups"`
				}{Groups: make([]groupResult, 0, len(records))}
				for _, record := range records {
					result.Groups = append(result.Groups, groupResult{
						GroupID: record.GroupID, Tag: record.Tag, Mode: record.Mode,
						ActiveTag: record.ActiveTag, Members: append([]string(nil), record.Members...),
						CooldownUntil: record.CooldownUntil, Revision: record.Revision,
					})
				}
				return clioutput.SetResult(cmd, result)
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
