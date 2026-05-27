package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/isksss/sqio/internal/history"
	"github.com/spf13/cobra"
)

// newHistoryCommand creates the command that prints locally persisted query
// history.
func newHistoryCommand() *cobra.Command {
	var format string
	var limit int
	cmd := &cobra.Command{
		Use:   "history",
		Short: "print query history",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			entries, err := history.New("").List(ctx, limit)
			if err != nil {
				return &CommandError{Type: "history", Message: err.Error(), Code: ExitInternal}
			}
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			for _, entry := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%dms\t%s\n", entry.ID, entry.Connection, entry.ElapsedMS, entry.SQL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum entries")
	return cmd
}
