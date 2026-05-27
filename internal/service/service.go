// Package service provides application-level operations shared by the CLI and
// TUI frontends.
package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/isksss/sqio/internal/db"
	"github.com/isksss/sqio/internal/output"
	"github.com/isksss/sqio/internal/query"
)

// Executor executes SQL through the configured database backend.
type Executor struct{}

// ExecOptions carries execution settings from the presentation layer into the
// service and database layers.
type ExecOptions struct {
	Format      string
	MaxRows     int
	Driver      string
	DSN         string
	Explain     bool
	Transaction bool
}

// Exec executes sql according to opts. Without a database connection it supports
// a small built-in "select 1" response for smoke tests and demo flows.
func (Executor) Exec(ctx context.Context, sql string, opts ExecOptions) (output.Result, error) {
	started := time.Now()
	statements := query.Statements(sql)
	if len(statements) == 0 {
		return output.Result{ElapsedMS: elapsed(started)}, nil
	}
	select {
	case <-ctx.Done():
		return output.Result{}, ctx.Err()
	default:
	}
	if opts.Driver != "" || opts.DSN != "" {
		return db.Execute(ctx, db.Config{Driver: opts.Driver, DSN: opts.DSN}, sql, db.ExecuteOptions{
			MaxRows: opts.MaxRows, Explain: opts.Explain, Transaction: opts.Transaction,
		})
	}
	if len(statements) == 1 && strings.EqualFold(statements[0], "select 1") {
		return output.Result{
			Columns:   []string{"?column?"},
			Rows:      [][]interface{}{{1}},
			RowCount:  1,
			ElapsedMS: elapsed(started),
		}, nil
	}
	return output.Result{}, fmt.Errorf("database connection is required for this SQL")
}

// Write executes sql and writes the result directly to w when a database-backed
// connection is available. Demo-mode SQL still uses the in-memory result path.
func (Executor) Write(ctx context.Context, w io.Writer, sql string, opts ExecOptions) (output.Result, error) {
	if opts.Transaction {
		result, err := Executor{}.Exec(ctx, sql, opts)
		if err != nil {
			return output.Result{}, err
		}
		if err := output.Write(w, opts.Format, result); err != nil {
			return output.Result{}, err
		}
		return result, nil
	}
	if opts.Driver != "" || opts.DSN != "" {
		return db.ExecuteToWriter(ctx, db.Config{Driver: opts.Driver, DSN: opts.DSN}, sql, db.ExecuteOptions{
			MaxRows: opts.MaxRows, Explain: opts.Explain, Transaction: opts.Transaction,
		}, w, opts.Format)
	}
	result, err := Executor{}.Exec(ctx, sql, opts)
	if err != nil {
		return output.Result{}, err
	}
	if err := output.Write(w, opts.Format, result); err != nil {
		return output.Result{}, err
	}
	return result, nil
}

// elapsed returns milliseconds since started.
func elapsed(started time.Time) int64 {
	return time.Since(started).Milliseconds()
}
