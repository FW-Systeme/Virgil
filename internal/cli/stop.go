package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pm, ok := pmFromCtx(cmd.Context())
			if !ok {
				return fmt.Errorf("process manager not initialized")
			}

			if err := pm.StopProcess(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Stopped app %q\n", args[0])
			return nil
		},
	}
}
