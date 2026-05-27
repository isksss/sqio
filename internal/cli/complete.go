package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type completeOptions struct {
	connectionOptions
	prefix string
	table  string
	format string
}

func newCompleteCommand() *cobra.Command {
	var opts completeOptions
	cmd := &cobra.Command{
		Use:   "complete",
		Short: "print SQL completion candidates",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			metadata, cleanup, err := metadataService(ctx, cfg, opts.connectionOptions)
			if err != nil {
				return err
			}
			defer cleanup()
			candidates, err := metadata.Complete(ctx, opts.prefix, opts.table)
			if err != nil {
				return &CommandError{Type: "metadata", Message: err.Error(), Code: ExitInternal}
			}
			switch opts.format {
			case "json":
				return json.NewEncoder(cmd.OutOrStdout()).Encode(candidates)
			case "", "table":
				for _, candidate := range candidates {
					if candidate.Table != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", candidate.Value, candidate.Kind, candidate.Table)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", candidate.Value, candidate.Kind)
					}
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported completion format: " + opts.format, Code: ExitInternal}
			}
		},
	}
	cmd.Flags().StringVar(&opts.prefix, "prefix", "", "SQL text before the cursor")
	cmd.Flags().StringVar(&opts.table, "table", "", "limit column candidates to one table")
	cmd.Flags().StringVar(&opts.format, "format", "table", "output format")
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	return cmd
}
