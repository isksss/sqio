package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/isksss/sqio/internal/output"
	"github.com/isksss/sqio/internal/query"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Config struct {
	Driver string
	DSN    string
}

type ExecuteOptions struct {
	MaxRows     int
	Explain     bool
	Transaction bool
}

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

func elapsed(started time.Time) int64 {
	return time.Since(started).Milliseconds()
}
