package service

import (
	"context"
	"io"

	"github.com/isksss/sqio/internal/db"
	"github.com/isksss/sqio/internal/output"
)

// DumpOptions controls table export.
type DumpOptions struct {
	Driver  string
	DSN     string
	Table   string
	Format  string
	MaxRows int
}

// LoadOptions controls table import.
type LoadOptions struct {
	Driver string
	DSN    string
	Table  string
	Format string
}

// DumpTable writes rows from one database table to w.
func DumpTable(ctx context.Context, w io.Writer, opts DumpOptions) (output.Result, error) {
	return db.DumpTable(ctx, db.Config{Driver: opts.Driver, DSN: opts.DSN}, opts.Table, db.ExecuteOptions{MaxRows: opts.MaxRows}, w, opts.Format)
}

// LoadTable reads rows from r into one database table.
func LoadTable(ctx context.Context, r io.Reader, opts LoadOptions) (db.ImportResult, error) {
	switch opts.Format {
	case "", "csv":
		return db.LoadCSV(ctx, db.Config{Driver: opts.Driver, DSN: opts.DSN}, opts.Table, r)
	case "json":
		return db.LoadJSON(ctx, db.Config{Driver: opts.Driver, DSN: opts.DSN}, opts.Table, r)
	case "jsonl":
		return db.LoadJSONL(ctx, db.Config{Driver: opts.Driver, DSN: opts.DSN}, opts.Table, r)
	case "yaml":
		return db.LoadYAML(ctx, db.Config{Driver: opts.Driver, DSN: opts.DSN}, opts.Table, r)
	default:
		return db.ImportResult{}, ErrUnsupportedImportFormat(opts.Format)
	}
}

// ErrUnsupportedImportFormat formats unsupported table import formats.
func ErrUnsupportedImportFormat(format string) error {
	return errUnsupportedImportFormat(format)
}
