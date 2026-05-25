package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/isksss/sqio/internal/db"
	"github.com/isksss/sqio/internal/output"
	"github.com/isksss/sqio/internal/query"
)

type Executor struct{}

type ExecOptions struct {
	Format      string
	MaxRows     int
	Driver      string
	DSN         string
	Explain     bool
	Transaction bool
}

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

func elapsed(started time.Time) int64 {
	return time.Since(started).Milliseconds()
}
