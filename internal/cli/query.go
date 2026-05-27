package cli

import (
	"os"

	"github.com/isksss/sqio/internal/editor"
	"github.com/isksss/sqio/internal/picker"
	"github.com/isksss/sqio/internal/query"
	"github.com/spf13/cobra"
)

// newQueryCommand creates the editor-assisted SQL execution command.
func newQueryCommand() *cobra.Command {
	var opts execOptions
	var pick bool
	cmd := &cobra.Command{
		Use:   "query",
		Short: "edit and execute SQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			sql, err := query.Read(query.Source{SQL: opts.sql, File: opts.file, In: os.Stdin})
			if err != nil {
				return &CommandError{Type: "input", Message: err.Error(), Code: ExitInternal}
			}
			if pick {
				files, err := picker.SQLFiles(".")
				if err != nil {
					return &CommandError{Type: "input", Message: err.Error(), Code: ExitInternal}
				}
				selected, err := picker.Pick(files)
				if err != nil {
					return &CommandError{Type: "input", Message: err.Error(), Code: ExitInternal}
				}
				b, err := os.ReadFile(selected)
				if err != nil {
					return &CommandError{Type: "input", Message: err.Error(), Code: ExitInternal}
				}
				sql = string(b)
			}
			if sql == "" {
				sql, err = editor.Edit("")
				if err != nil {
					return &CommandError{Type: "editor", Message: err.Error(), Code: ExitInternal}
				}
			}
			return runExec(cmd, opts, sql)
		},
	}
	cmd.Flags().StringVar(&opts.sql, "sql", "", "SQL string")
	cmd.Flags().StringVar(&opts.file, "file", "", "SQL file")
	cmd.Flags().BoolVar(&pick, "pick", false, "pick a SQL file with fzf or fallback picker")
	cmd.Flags().StringVar(&opts.format, "format", "", "output format")
	cmd.Flags().StringVar(&opts.out, "out", "", "output file")
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	cmd.Flags().StringVar(&opts.timeout, "timeout", "", "query timeout")
	cmd.Flags().IntVar(&opts.maxRows, "max-rows", 0, "maximum rows")
	cmd.Flags().IntVar(&opts.maxBytes, "max-bytes", 0, "maximum output bytes")
	cmd.Flags().BoolVar(&opts.noHistory, "no-history", false, "disable query history")
	cmd.Flags().BoolVar(&opts.explain, "explain", false, "run EXPLAIN for SQL")
	cmd.Flags().BoolVar(&opts.transaction, "transaction", false, "execute SQL in a transaction")
	return cmd
}
