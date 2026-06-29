package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a registered app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pm, ok := pmFromCtx(cmd.Context())
			if !ok {
				return fmt.Errorf("process manager not initialized")
			}

			if err := pm.RemoveProcess(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed app %q\n", args[0])
			return nil
		},
	}
}
