package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/isksss/sqio/internal/linter"
	"github.com/isksss/sqio/internal/query"
	"github.com/spf13/cobra"
)

// lintOptions contains flags for SQL linting.
type lintOptions struct {
	sql     string
	file    string
	format  string
	level   string
	enable  []string
	disable []string
}

// newLintCommand creates the SQL linter command.
func newLintCommand() *cobra.Command {
	var opts lintOptions
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "lint SQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if opts.level == "" {
				opts.level = cfg.Lint.Level
			}
			sql, err := query.Read(query.Source{SQL: opts.sql, File: opts.file, In: os.Stdin})
			if err != nil {
				return &CommandError{Type: "input", Message: err.Error(), Code: 2}
			}
			result := linter.Lint(sql, linter.Options{Level: opts.level, Enable: opts.enable, Disable: opts.disable})
			if opts.format == "json" {
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
					return &CommandError{Type: "output", Message: err.Error(), Code: 2}
				}
			} else {
				for _, issue := range result.Issues {
					fmt.Fprintf(cmd.OutOrStdout(), "%s line=%d %s %s\n", strings.ToUpper(issue.Severity), issue.Line, issue.Rule, issue.Message)
				}
			}
			if len(result.Issues) > 0 {
				return &CommandError{Type: "lint_issue", Message: "lint issues found", Code: 1}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.sql, "sql", "", "SQL string")
	cmd.Flags().StringVar(&opts.file, "file", "", "SQL file")
	cmd.Flags().StringVar(&opts.format, "format", "human", "output format")
	cmd.Flags().StringVar(&opts.level, "level", "", "minimum severity")
	cmd.Flags().StringSliceVar(&opts.enable, "enable", nil, "enable lint rules")
	cmd.Flags().StringSliceVar(&opts.disable, "disable", nil, "disable lint rules")
	return cmd
}
