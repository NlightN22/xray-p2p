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
		if contract.successResult != nil || jsonQuietFlagCommands[cmd.CommandPath()] ||
			jsonLegacyQuietCommands[cmd.CommandPath()] {
			runE := cmd.RunE
			run := cmd.Run
			cmd.Run = nil
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				if enabled() {
					if quiet := cmd.Flags().Lookup("quiet"); quiet != nil {
						if err := cmd.Flags().Set("quiet", "true"); err != nil {
							return err
						}
					} else if jsonLegacyQuietCommands[cmd.CommandPath()] {
						args = append(append([]string(nil), args...), "--quiet")
					}
				}
				if runE != nil {
					if err := runE(cmd, args); err != nil {
						return err
					}
				} else if run != nil {
					run(cmd, args)
				}
				if enabled() && contract.successResult != nil && !clioutput.HasResult(cmd) {
					return clioutput.SetResult(cmd, contract.successResult(cmd, args))
				}
				return nil
			}
		}
		clioutput.WrapJSON(cmd, enabled)
		clioutput.RejectJSON(cmd, enabled)
	}
	visit(root)
}
