package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			pm, ok := pmFromCtx(cmd.Context())
			if !ok {
				return fmt.Errorf("process manager not initialized")
			}

			processes, err := pm.ListProcesses(cmd.Context())
			if err != nil {
				return err
			}

			if len(processes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No apps registered")
				return nil
			}

			for _, p := range processes {
				status := "active"
				if !p.Enabled {
					status = "disabled"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s  %-6s  port %-5d  %s\n", p.Name, p.Type, p.Port, status)
			}
			return nil
		},
	}
}
