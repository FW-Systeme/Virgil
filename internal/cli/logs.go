package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var lines int
	var follow bool
	var output string

	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "Show app logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pm, ok := pmFromCtx(cmd.Context())
			if !ok {
				return fmt.Errorf("process manager not initialized")
			}
			name := args[0]
			r, err := pm.Logs(cmd.Context(), name, lines, follow)
			if err != nil {
				return err
			}
			defer r.Close()

			if output != "" {
				f, err := os.Create(output)
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = io.Copy(io.MultiWriter(cmd.OutOrStdout(), f), r)
				return err
			}
			_, err = io.Copy(cmd.OutOrStdout(), r)
			return err
		},
	}

	cmd.Flags().IntVarP(&lines, "lines", "n", 50, "Number of past lines (0 = all)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (tail -f)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Also write output to file")

	return cmd
}
