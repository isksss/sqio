package cli

import "github.com/spf13/cobra"

func newExplainCommand() *cobra.Command {
	var opts execOptions
	opts.explain = true
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "explain a SQL execution plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.explain = true
			return runExec(cmd, opts, "")
		},
	}
	cmd.Flags().StringVar(&opts.sql, "sql", "", "SQL string")
	cmd.Flags().StringVar(&opts.file, "file", "", "SQL file")
	cmd.Flags().StringVar(&opts.format, "format", "", "output format")
	cmd.Flags().StringVar(&opts.out, "out", "", "output file")
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	cmd.Flags().StringVar(&opts.timeout, "timeout", "", "query timeout")
	cmd.Flags().IntVar(&opts.maxRows, "max-rows", 0, "maximum rows")
	cmd.Flags().IntVar(&opts.maxBytes, "max-bytes", 0, "maximum output bytes")
	cmd.Flags().BoolVar(&opts.noHistory, "no-history", false, "disable query history")
	cmd.Flags().BoolVar(&opts.analyze, "analyze", false, "run EXPLAIN ANALYZE when supported")
	return cmd
}
