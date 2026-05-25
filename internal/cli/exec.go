package cli

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/isksss/sqio/internal/history"
	"github.com/isksss/sqio/internal/output"
	"github.com/isksss/sqio/internal/query"
	"github.com/isksss/sqio/internal/service"
	"github.com/spf13/cobra"
)

type execOptions struct {
	connectionOptions
	sql         string
	file        string
	format      string
	out         string
	timeout     string
	maxRows     int
	maxBytes    int
	noHistory   bool
	explain     bool
	transaction bool
}

func newExecCommand() *cobra.Command {
	var opts execOptions
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "execute SQL",
		RunE:  func(cmd *cobra.Command, args []string) error { return runExec(cmd, opts, "") },
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
	cmd.Flags().BoolVar(&opts.explain, "explain", false, "run EXPLAIN for SQL")
	cmd.Flags().BoolVar(&opts.transaction, "transaction", false, "execute SQL in a transaction")
	return cmd
}

func runExec(cmd *cobra.Command, opts execOptions, sqlOverride string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if opts.format == "" {
		opts.format = cfg.Query.Format
	}
	if opts.timeout == "" {
		opts.timeout = cfg.Query.Timeout
	}
	if opts.maxRows == 0 {
		opts.maxRows = cfg.Query.MaxRows
	}
	sql := sqlOverride
	if sql == "" {
		sql, err = query.Read(query.Source{SQL: opts.sql, File: opts.file, In: os.Stdin})
		if err != nil {
			return &CommandError{Type: "input", Message: err.Error(), Code: ExitInternal}
		}
	}
	if readonlyEnabled(cfg, opts.connectionOptions) {
		if danger, ok := query.Dangerous(sql); ok {
			return &CommandError{Type: "readonly", Message: danger.Message, Code: ExitSQLSyntax}
		}
		if query.Mutating(sql) {
			return &CommandError{Type: "readonly", Message: "readonly connection rejected mutating SQL", Code: ExitSQLSyntax}
		}
	}
	timeout, err := time.ParseDuration(opts.timeout)
	if err != nil {
		return &CommandError{Type: "config", Message: err.Error(), Code: ExitInternal}
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()
	driver, dsn, cleanup, err := prepareConnection(ctx, cfg, opts.connectionOptions)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	result, err := service.Executor{}.Exec(ctx, sql, service.ExecOptions{
		Format: opts.format, MaxRows: opts.maxRows, Driver: driver, DSN: dsn,
		Explain: opts.explain, Transaction: opts.transaction,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return &CommandError{Type: "timeout", Message: err.Error(), Code: ExitTimeout}
		}
		return &CommandError{Type: "internal", Message: err.Error(), Code: ExitInternal}
	}
	w := cmd.OutOrStdout()
	var outFile *os.File
	if opts.out != "" {
		outFile, err = os.Create(opts.out)
		if err != nil {
			return &CommandError{Type: "output", Message: err.Error(), Code: ExitInternal}
		}
		defer outFile.Close()
		w = outFile
	}
	if opts.maxBytes > 0 {
		w = &output.LimitWriter{Writer: w, Limit: opts.maxBytes}
	}
	if err := output.Write(w, opts.format, result); err != nil {
		return &CommandError{Type: "output", Message: err.Error(), Code: ExitInternal}
	}
	if !opts.noHistory {
		_ = history.New("").Append(cmd.Context(), history.Entry{
			SQL:        sql,
			Connection: opts.conn,
			ElapsedMS:  result.ElapsedMS,
		})
	}
	return nil
}
