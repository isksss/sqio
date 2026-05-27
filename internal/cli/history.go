package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/isksss/sqio/internal/history"
	"github.com/spf13/cobra"
)

// newHistoryCommand creates the command that prints locally persisted query
// history.
func newHistoryCommand() *cobra.Command {
	var format string
	var limit int
	var search string
	var connName string
	var favorite bool
	var tags string
	cmd := &cobra.Command{
		Use:   "history",
		Short: "print query history",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			entries, err := history.New("").ListWithOptions(ctx, history.ListOptions{
				Limit: limit, Search: search, Connection: connName, Favorite: favorite, Tags: tags,
			})
			if err != nil {
				return &CommandError{Type: "history", Message: err.Error(), Code: ExitInternal}
			}
			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			case "", "table":
				for _, entry := range entries {
					fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%d rows\t%dms\t%s\t%s\t%s\n", entry.ID, entry.Connection, historyStatus(entry), entry.RowCount, entry.ElapsedMS, favoriteLabel(entry.Favorite), entry.Tags, entry.SQL)
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported history format: " + format, Code: ExitInternal}
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum entries")
	cmd.Flags().StringVar(&search, "search", "", "filter SQL text")
	cmd.Flags().StringVar(&connName, "conn", "", "filter connection name")
	cmd.Flags().BoolVar(&favorite, "favorite", false, "show favorite entries only")
	cmd.Flags().StringVar(&tags, "tags", "", "filter tags")
	cmd.AddCommand(
		newHistoryFavoriteCommand(true),
		newHistoryFavoriteCommand(false),
		newHistoryTagCommand(),
		newHistoryRunCommand(),
	)
	return cmd
}

func newHistoryFavoriteCommand(favorite bool) *cobra.Command {
	use := "favorite ID"
	short := "mark a history entry as favorite"
	if !favorite {
		use = "unfavorite ID"
		short = "remove favorite from a history entry"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseHistoryID(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if err := history.New("").SetFavorite(ctx, id, favorite); err != nil {
				return &CommandError{Type: "history", Message: err.Error(), Code: ExitInternal}
			}
			if !global.quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\n", id, favoriteLabel(favorite))
			}
			return nil
		},
	}
	return cmd
}

func newHistoryTagCommand() *cobra.Command {
	var tags string
	cmd := &cobra.Command{
		Use:   "tag ID",
		Short: "set tags on a history entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseHistoryID(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if err := history.New("").SetTags(ctx, id, tags); err != nil {
				return &CommandError{Type: "history", Message: err.Error(), Code: ExitInternal}
			}
			if !global.quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\n", id, tags)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&tags, "tags", "", "tags to store")
	return cmd
}

func newHistoryRunCommand() *cobra.Command {
	var opts execOptions
	cmd := &cobra.Command{
		Use:   "run ID",
		Short: "run SQL from history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseHistoryID(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			entry, err := history.New("").Get(ctx, id)
			if err != nil {
				return &CommandError{Type: "history", Message: err.Error(), Code: ExitInternal}
			}
			if opts.conn == "" {
				opts.conn = entry.Connection
			}
			return runExec(cmd, opts, entry.SQL)
		},
	}
	cmd.Flags().StringVar(&opts.format, "format", "", "output format")
	cmd.Flags().StringVar(&opts.out, "out", "", "output file")
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	cmd.Flags().StringVar(&opts.timeout, "timeout", "", "query timeout")
	cmd.Flags().IntVar(&opts.maxRows, "max-rows", 0, "maximum rows")
	cmd.Flags().IntVar(&opts.maxBytes, "max-bytes", 0, "maximum output bytes")
	cmd.Flags().BoolVar(&opts.noHistory, "no-history", false, "disable query history")
	cmd.Flags().BoolVar(&opts.explain, "explain", false, "run EXPLAIN for SQL")
	cmd.Flags().BoolVar(&opts.analyze, "analyze", false, "run EXPLAIN ANALYZE when supported")
	cmd.Flags().BoolVar(&opts.transaction, "transaction", false, "execute SQL in a transaction")
	return cmd
}

func parseHistoryID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, &CommandError{Type: "input", Message: "history ID must be a positive integer", Code: ExitInternal}
	}
	return id, nil
}

func favoriteLabel(favorite bool) string {
	if favorite {
		return "favorite"
	}
	return "-"
}

func historyStatus(entry history.Entry) string {
	if entry.Success {
		return "ok"
	}
	if entry.Error == "" {
		return "error"
	}
	return "error: " + entry.Error
}
