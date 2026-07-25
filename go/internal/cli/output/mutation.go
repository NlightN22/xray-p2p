package output

import "github.com/spf13/cobra"

// MutationResult is the common typed result for payload-free mutations.
type MutationResult struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	Entity    string `json:"entity"`
}

// WrapMutationResult publishes a typed result after the command handler succeeds.
func WrapMutationResult(
	cmd *cobra.Command,
	operation string,
	entity func(*cobra.Command, []string) string,
) {
	runE := cmd.RunE
	run := cmd.Run
	cmd.Run = nil
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if runE != nil {
			if err := runE(cmd, args); err != nil {
				return err
			}
		} else if run != nil {
			run(cmd, args)
		}
		if !Enabled(cmd) {
			return nil
		}
		return SetResult(cmd, MutationResult{
			Status:    "completed",
			Operation: operation,
			Entity:    entity(cmd, args),
		})
	}
}
