// Package db opens database connections and executes SQL against supported
// drivers.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/isksss/sqio/internal/output"
	"github.com/isksss/sqio/internal/query"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// Config contains the normalized driver name and DSN needed to open a database.
type Config struct {
	Driver string
	DSN    string
}

// ExecuteOptions controls optional execution behavior for a SQL request.
type ExecuteOptions struct {
	MaxRows     int
	Explain     bool
	Transaction bool
}

// Open validates cfg, opens the matching database driver, and verifies the
// connection with PingContext. The returned driver is the normalized driver name.
func Open(ctx context.Context, cfg Config) (*sql.DB, string, error) {
	driver, dsn, err := normalize(cfg)
	if err != nil {
		return nil, "", err
	}
	conn, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, "", err
	}
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, "", err
	}
	return conn, driver, nil
}

// Execute runs each parsed statement in sqlText and returns the result from the
// last statement. Row-returning statements are scanned into output.Result, while
// write statements report affected row count.
func Execute(ctx context.Context, cfg Config, sqlText string, opts ExecuteOptions) (output.Result, error) {
	started := time.Now()
	conn, driver, err := Open(ctx, cfg)
	if err != nil {
		return output.Result{}, err
	}
	defer conn.Close()

	if opts.Explain {
		sqlText = explainSQL(driver, sqlText)
	}
	var execer interface {
		QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
		ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	} = conn
	var tx *sql.Tx
	if opts.Transaction {
		tx, err = conn.BeginTx(ctx, nil)
		if err != nil {
			return output.Result{}, err
		}
		defer tx.Rollback()
		execer = tx
	}
	result := output.Result{ElapsedMS: elapsed(started)}
	for _, statement := range query.Statements(sqlText) {
		if returnsRows(statement) {
			rows, err := execer.QueryContext(ctx, statement)
			if err != nil {
				return output.Result{}, err
			}
			scanned, scanErr := scanRows(rows, opts.MaxRows)
			_ = rows.Close()
			if scanErr != nil {
				return output.Result{}, scanErr
			}
			scanned.ElapsedMS = elapsed(started)
			result = scanned
			continue
		}
		execResult, execErr := execer.ExecContext(ctx, statement)
		if execErr != nil {
			return output.Result{}, execErr
		}
		affected, _ := execResult.RowsAffected()
		result = output.Result{RowCount: int(affected), ElapsedMS: elapsed(started)}
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return output.Result{}, err
		}
	}
	return result, nil
}

// ExecuteToWriter runs SQL and streams the last row-returning statement directly
// to w. Earlier statements are executed for side effects only, matching Execute's
// "last statement is the result" behavior.
func ExecuteToWriter(ctx context.Context, cfg Config, sqlText string, opts ExecuteOptions, w io.Writer, format string) (output.Result, error) {
	started := time.Now()
	conn, driver, err := Open(ctx, cfg)
	if err != nil {
		return output.Result{}, err
	}
	defer conn.Close()

	if opts.Explain {
		sqlText = explainSQL(driver, sqlText)
	}
	statements := query.Statements(sqlText)
	if len(statements) == 0 {
		result := output.Result{ElapsedMS: elapsed(started)}
		if err := output.Write(w, format, result); err != nil {
			return output.Result{}, err
		}
		return result, nil
	}
	var execer interface {
		QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
		ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	} = conn
	var tx *sql.Tx
	if opts.Transaction {
		tx, err = conn.BeginTx(ctx, nil)
		if err != nil {
			return output.Result{}, err
		}
		defer tx.Rollback()
		execer = tx
	}
	result := output.Result{ElapsedMS: elapsed(started)}
	for i, statement := range statements {
		last := i == len(statements)-1
		if returnsRows(statement) {
			rows, err := execer.QueryContext(ctx, statement)
			if err != nil {
				return output.Result{}, err
			}
			if last {
				result, err = streamRows(rows, opts.MaxRows, w, format, started)
			} else {
				result, err = scanRows(rows, opts.MaxRows)
			}
			_ = rows.Close()
			if err != nil {
				return output.Result{}, err
			}
			result.ElapsedMS = elapsed(started)
			continue
		}
		execResult, execErr := execer.ExecContext(ctx, statement)
		if execErr != nil {
			return output.Result{}, execErr
		}
		affected, _ := execResult.RowsAffected()
		result = output.Result{RowCount: int(affected), ElapsedMS: elapsed(started)}
		if last {
			if err := output.Write(w, format, result); err != nil {
				return output.Result{}, err
			}
		}
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return output.Result{}, err
		}
	}
	return result, nil
}

// returnsRows reports whether statement should be executed with QueryContext
// instead of ExecContext based on its first SQL token.
func returnsRows(statement string) bool {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(statement)))
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "select", "with", "show", "describe", "explain", "pragma":
		return true
	default:
		return false
	}
}

// normalize maps user-facing driver aliases onto database/sql driver names and
// validates that a DSN is present.
func normalize(cfg Config) (string, string, error) {
	driver := strings.ToLower(cfg.Driver)
	switch driver {
	case "sqlite", "sqlite3":
		if cfg.DSN == "" {
			return "", "", fmt.Errorf("sqlite requires dsn or database path")
		}
		return "sqlite", cfg.DSN, nil
	case "postgres", "postgresql", "pgx":
		if cfg.DSN == "" {
			return "", "", fmt.Errorf("postgres requires dsn")
		}
		return "pgx", cfg.DSN, nil
	case "mysql":
		if cfg.DSN == "" {
			return "", "", fmt.Errorf("mysql requires dsn")
		}
		return "mysql", cfg.DSN, nil
	default:
		return "", "", fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}
}

// scanRows copies database rows into a driver-neutral output.Result, converting
// byte slices to strings for readable CLI output.
func scanRows(rows *sql.Rows, maxRows int) (output.Result, error) {
	columns, err := rows.Columns()
	if err != nil {
		return output.Result{}, err
	}
	result := output.Result{Columns: columns}
	values := make([]interface{}, len(columns))
	dest := make([]interface{}, len(columns))
	for i := range values {
		dest[i] = &values[i]
	}
	for rows.Next() {
		if maxRows > 0 && len(result.Rows) >= maxRows {
			break
		}
		for i := range values {
			values[i] = nil
		}
		if err := rows.Scan(dest...); err != nil {
			return output.Result{}, err
		}
		row := make([]interface{}, len(columns))
		for i, value := range values {
			if bytes, ok := value.([]byte); ok {
				row[i] = string(bytes)
			} else {
				row[i] = value
			}
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return output.Result{}, err
	}
	result.RowCount = len(result.Rows)
	return result, nil
}

func streamRows(rows *sql.Rows, maxRows int, w io.Writer, format string, started time.Time) (output.Result, error) {
	columns, err := rows.Columns()
	if err != nil {
		return output.Result{}, err
	}
	values := make([]interface{}, len(columns))
	dest := make([]interface{}, len(columns))
	for i := range values {
		dest[i] = &values[i]
	}
	writer, err := output.NewStreamWriter(w, format, columns, func() int64 { return elapsed(started) })
	if err != nil {
		return output.Result{}, err
	}
	for rows.Next() {
		if maxRows > 0 && writer.RowCount() >= maxRows {
			break
		}
		for i := range values {
			values[i] = nil
		}
		if err := rows.Scan(dest...); err != nil {
			return output.Result{}, err
		}
		row := make([]interface{}, len(columns))
		for i, value := range values {
			if bytes, ok := value.([]byte); ok {
				row[i] = string(bytes)
			} else {
				row[i] = value
			}
		}
		if err := writer.WriteRow(row); err != nil {
			return output.Result{}, err
		}
	}
	if err := rows.Err(); err != nil {
		return output.Result{}, err
	}
	return writer.Close()
}

// explainSQL prefixes each statement with the driver's EXPLAIN syntax.
func explainSQL(driver, sqlText string) string {
	prefix := "EXPLAIN "
	if driver == "sqlite" {
		prefix = "EXPLAIN QUERY PLAN "
	}
	statements := query.Statements(sqlText)
	for i, statement := range statements {
		statements[i] = prefix + statement
	}
	return strings.Join(statements, ";")
}

// elapsed returns milliseconds since started.
func elapsed(started time.Time) int64 {
	return time.Since(started).Milliseconds()
}
