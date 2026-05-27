package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/isksss/sqio/internal/history"
	"github.com/isksss/sqio/internal/output"
	"github.com/isksss/sqio/internal/query"
	"github.com/isksss/sqio/internal/service"
	"github.com/spf13/cobra"
)

// execOptions contains flags for commands that execute SQL.
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
	auditLog    string
	explain     bool
	analyze     bool
	transaction bool
}

type auditEntry struct {
	ExecutedAt time.Time `json:"executed_at"`
	SQL        string    `json:"sql"`
	Connection string    `json:"connection"`
	Driver     string    `json:"driver"`
	ElapsedMS  int64     `json:"elapsed_ms"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	RowCount   int       `json:"row_count"`
}

// newExecCommand creates the non-interactive SQL execution command.
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
	cmd.Flags().StringVar(&opts.auditLog, "audit-log", "", "append execution audit log as JSONL")
	cmd.Flags().BoolVar(&opts.explain, "explain", false, "run EXPLAIN for SQL")
	cmd.Flags().BoolVar(&opts.analyze, "analyze", false, "run EXPLAIN ANALYZE when supported")
	cmd.Flags().BoolVar(&opts.transaction, "transaction", false, "execute SQL in a transaction")
	return cmd
}

// runExec resolves SQL input, connection settings, execution options, output
// targets, and history persistence for exec-like commands.
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
	started := time.Now()
	result, err := service.Executor{}.Write(ctx, w, sql, service.ExecOptions{
		Format: opts.format, MaxRows: opts.maxRows, Driver: driver, DSN: dsn,
		Explain: opts.explain, Analyze: opts.analyze, Transaction: opts.transaction,
	})
	if err != nil {
		result := output.Result{ElapsedMS: time.Since(started).Milliseconds()}
		appendExecHistory(cmd, opts, sql, driver, result, err)
		if auditErr := appendExecAudit(opts, sql, driver, result, err); auditErr != nil {
			return &CommandError{Type: "output", Message: auditErr.Error(), Code: ExitInternal}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return &CommandError{Type: "timeout", Message: err.Error(), Code: ExitTimeout}
		}
		return &CommandError{Type: "output", Message: err.Error(), Code: ExitInternal}
	}
	appendExecHistory(cmd, opts, sql, driver, result, nil)
	if err := appendExecAudit(opts, sql, driver, result, nil); err != nil {
		return &CommandError{Type: "output", Message: err.Error(), Code: ExitInternal}
	}
	return nil
}

func appendExecHistory(cmd *cobra.Command, opts execOptions, sql, driver string, result output.Result, execErr error) {
	if !opts.noHistory {
		errorText := ""
		if execErr != nil {
			errorText = strings.TrimSpace(execErr.Error())
		}
		_ = history.New("").Append(cmd.Context(), history.Entry{
			SQL:        sql,
			Connection: opts.conn,
			ElapsedMS:  result.ElapsedMS,
			Success:    execErr == nil,
			Error:      errorText,
			RowCount:   result.RowCount,
			Driver:     driver,
		})
	}
}

func appendExecAudit(opts execOptions, sql, driver string, result output.Result, execErr error) error {
	if opts.auditLog == "" {
		return nil
	}
	errorText := ""
	if execErr != nil {
		errorText = strings.TrimSpace(execErr.Error())
	}
	file, err := os.OpenFile(opts.auditLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	entry := auditEntry{
		ExecutedAt: time.Now().UTC(),
		SQL:        sql,
		Connection: opts.conn,
		Driver:     driver,
		ElapsedMS:  result.ElapsedMS,
		Success:    execErr == nil,
		Error:      errorText,
		RowCount:   result.RowCount,
	}
	if err := json.NewEncoder(file).Encode(entry); err != nil {
		return err
	}
	return nil
}
