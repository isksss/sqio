package cli

import (
	"bytes"
	"os"

	"github.com/isksss/sqio/internal/formatter"
	"github.com/isksss/sqio/internal/query"
	"github.com/spf13/cobra"
)

// fmtOptions contains flags for SQL formatting.
type fmtOptions struct {
	sql            string
	file           string
	write          bool
	check          bool
	dialect        string
	indent         int
	keywordCase    string
	identifierCase string
	lineWidth      int
}

// newFmtCommand creates the SQL formatter command.
func newFmtCommand() *cobra.Command {
	var opts fmtOptions
	cmd := &cobra.Command{
		Use:   "fmt",
		Short: "format SQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if opts.dialect == "" {
				opts.dialect = cfg.Formatter.Dialect
			}
			if opts.indent == 0 {
				opts.indent = cfg.Formatter.Indent
			}
			if opts.keywordCase == "" {
				opts.keywordCase = cfg.Formatter.KeywordCase
			}
			if opts.identifierCase == "" {
				opts.identifierCase = cfg.Formatter.IdentifierCase
			}
			if opts.lineWidth == 0 {
				opts.lineWidth = cfg.Formatter.LineWidth
			}
			sql, err := query.Read(query.Source{SQL: opts.sql, File: opts.file, In: os.Stdin})
			if err != nil {
				return &CommandError{Type: "input", Message: err.Error(), Code: ExitInternal}
			}
			formatted := formatter.Format(sql, formatter.Options{
				Dialect: opts.dialect, Indent: opts.indent, KeywordCase: opts.keywordCase,
				IdentifierCase: opts.identifierCase, LineWidth: opts.lineWidth,
			})
			if opts.check {
				if !bytes.Equal([]byte(sql), []byte(formatted)) {
					return &CommandError{Type: "format_check", Message: "SQL is not formatted", Code: ExitInternal}
				}
				return nil
			}
			if opts.write {
				if opts.file == "" {
					return &CommandError{Type: "input", Message: "--write requires --file", Code: ExitInternal}
				}
				if err := os.WriteFile(opts.file, []byte(formatted), 0o644); err != nil {
					return &CommandError{Type: "output", Message: err.Error(), Code: ExitInternal}
				}
				return nil
			}
			_, err = cmd.OutOrStdout().Write([]byte(formatted))
			return err
		},
	}
	cmd.Flags().StringVar(&opts.sql, "sql", "", "SQL string")
	cmd.Flags().StringVar(&opts.file, "file", "", "SQL file")
	cmd.Flags().BoolVar(&opts.write, "write", false, "write formatted SQL to file")
	cmd.Flags().BoolVar(&opts.check, "check", false, "check whether SQL is formatted")
	cmd.Flags().StringVar(&opts.dialect, "dialect", "", "SQL dialect")
	cmd.Flags().IntVar(&opts.indent, "indent", 0, "indent size")
	cmd.Flags().StringVar(&opts.keywordCase, "keyword-case", "", "keyword case")
	cmd.Flags().StringVar(&opts.identifierCase, "identifier-case", "", "identifier case")
	cmd.Flags().IntVar(&opts.lineWidth, "line-width", 0, "line width")
	return cmd
}
