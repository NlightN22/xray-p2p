package root

import (
	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
)

func classifyOutputContracts(root *cobra.Command) {
	oldSort := cobra.EnableCommandSorting
	cobra.EnableCommandSorting = false
	defer func() { cobra.EnableCommandSorting = oldSort }()

	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		children := cmd.Commands()
		if len(children) == 0 && (cmd.Run != nil || cmd.RunE != nil) {
			contract, ok := outputContractInventory[cmd.CommandPath()]
			if ok {
				clioutput.Classify(cmd, contract.class, contract.reason)
			}
		}
		for _, child := range children {
			visit(child)
		}
	}
	visit(root)
}

func decorateOutputContracts(root *cobra.Command, opts *rootOptions) {
	oldSort := cobra.EnableCommandSorting
	cobra.EnableCommandSorting = false
	defer func() { cobra.EnableCommandSorting = oldSort }()

	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			visit(child)
		}
		enabled := func() bool { return opts.jsonOutput }
		if validate := cmd.Args; validate != nil {
			cmd.Args = func(cmd *cobra.Command, args []string) error {
				err := validate(cmd, args)
				if err == nil || !enabled() {
					return err
				}
				_ = clioutput.WriteError(cmd.ErrOrStderr(), cmd.CommandPath(), "invalid_argument", err)
				return clioutput.MarkRendered(err)
			}
		}
		contract := outputContractInventory[cmd.CommandPath()]
		clioutput.WrapJSON(cmd, enabled, contract.defaultOperation)
		clioutput.RejectJSON(cmd, enabled)
	}
	visit(root)
}
