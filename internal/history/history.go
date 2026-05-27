// Package history stores executed SQL statements in a local SQLite database.
package history

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Entry represents one executed SQL statement persisted in history.
type Entry struct {
	ID         int64     `json:"id"`
	SQL        string    `json:"sql"`
	Connection string    `json:"connection"`
	ElapsedMS  int64     `json:"elapsed_ms"`
	ExecutedAt time.Time `json:"executed_at"`
	Tags       string    `json:"tags"`
	Favorite   bool      `json:"favorite"`
	Success    bool      `json:"success"`
	Error      string    `json:"error"`
	RowCount   int       `json:"row_count"`
	Driver     string    `json:"driver"`
}

// ListOptions filters history entries before returning them in newest-first
// order.
type ListOptions struct {
	Limit      int
	Search     string
	Connection string
	Favorite   bool
	Tags       string
}

// Store manages the SQLite-backed history database at a specific path.
type Store struct {
	path string
}

// New returns a Store using path, SQIO_HISTORY_PATH, or the default per-user
// history location in that order.
func New(path string) Store {
	if path == "" {
		path = os.Getenv("SQIO_HISTORY_PATH")
	}
	if path == "" {
		path = DefaultPath()
	}
	return Store{path: path}
}

// DefaultPath returns the conventional sqio history database path for the
// current user, falling back to the temporary directory when the home directory
// cannot be resolved.
func DefaultPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		_ = dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "sqio", "history.db")
	}
	return filepath.Join(home, ".local", "share", "sqio", "history.db")
}

// Append inserts entry into history, assigning the current UTC time when
// ExecutedAt is not already set.
func (s Store) Append(ctx context.Context, entry Entry) error {
	conn, err := s.open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if entry.ExecutedAt.IsZero() {
		entry.ExecutedAt = time.Now().UTC()
	}
	if !entry.Success && entry.Error == "" {
		entry.Success = true
	}
	_, err = conn.ExecContext(ctx, `insert into history (sql, connection, elapsed_ms, executed_at, tags, favorite, success, error, row_count, driver) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.SQL, entry.Connection, entry.ElapsedMS, entry.ExecutedAt.Format(time.RFC3339Nano), entry.Tags, entry.Favorite, entry.Success, entry.Error, entry.RowCount, entry.Driver)
	return err
}

// List returns recent history entries in reverse insertion order. A non-positive
// limit uses the default maximum of one hundred rows.
func (s Store) List(ctx context.Context, limit int) ([]Entry, error) {
	return s.ListWithOptions(ctx, ListOptions{Limit: limit})
}

// ListWithOptions returns recent history entries matching opts. A non-positive
// limit uses the default maximum of one hundred rows.
func (s Store) ListWithOptions(ctx context.Context, opts ListOptions) ([]Entry, error) {
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	conn, err := s.open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	where, args := historyWhere(opts)
	args = append(args, opts.Limit)
	rows, err := conn.QueryContext(ctx, `select id, sql, connection, elapsed_ms, executed_at, tags, favorite, success, error, row_count, driver from history`+where+` order by id desc limit ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// Get returns one history entry by id.
func (s Store) Get(ctx context.Context, id int64) (Entry, error) {
	conn, err := s.open(ctx)
	if err != nil {
		return Entry{}, err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, `select id, sql, connection, elapsed_ms, executed_at, tags, favorite, success, error, row_count, driver from history where id = ?`, id)
	if err != nil {
		return Entry{}, err
	}
	defer rows.Close()
	entries, err := scanEntries(rows)
	if err != nil {
		return Entry{}, err
	}
	if len(entries) == 0 {
		return Entry{}, fmt.Errorf("history entry not found: %d", id)
	}
	return entries[0], nil
}

// SetFavorite updates the favorite flag for one history entry.
func (s Store) SetFavorite(ctx context.Context, id int64, favorite bool) error {
	return s.update(ctx, `update history set favorite = ? where id = ?`, favorite, id)
}

// SetTags updates the free-form tag string for one history entry.
func (s Store) SetTags(ctx context.Context, id int64, tags string) error {
	return s.update(ctx, `update history set tags = ? where id = ?`, tags, id)
}

func (s Store) update(ctx context.Context, query string, args ...interface{}) error {
	conn, err := s.open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	result, err := conn.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("history entry not found")
	}
	return nil
}

func historyWhere(opts ListOptions) (string, []interface{}) {
	clauses := []string{}
	args := []interface{}{}
	if opts.Search != "" {
		clauses = append(clauses, "sql like ?")
		args = append(args, "%"+opts.Search+"%")
	}
	if opts.Connection != "" {
		clauses = append(clauses, "connection = ?")
		args = append(args, opts.Connection)
	}
	if opts.Favorite {
		clauses = append(clauses, "favorite = 1")
	}
	if opts.Tags != "" {
		clauses = append(clauses, "tags like ?")
		args = append(args, "%"+opts.Tags+"%")
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " where " + strings.Join(clauses, " and "), args
}

func scanEntries(rows *sql.Rows) ([]Entry, error) {
	entries := []Entry{}
	for rows.Next() {
		var entry Entry
		var executedAt string
		if err := rows.Scan(&entry.ID, &entry.SQL, &entry.Connection, &entry.ElapsedMS, &executedAt, &entry.Tags, &entry.Favorite, &entry.Success, &entry.Error, &entry.RowCount, &entry.Driver); err != nil {
			return nil, err
		}
		entry.ExecutedAt, _ = time.Parse(time.RFC3339Nano, executedAt)
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// open creates the history directory, opens the SQLite database, and ensures
// the schema exists before returning the connection.
func (s Store) open(ctx context.Context) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return nil, err
	}
	conn, err := sql.Open("sqlite", s.path)
	if err != nil {
		return nil, err
	}
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, `create table if not exists history (
id integer primary key autoincrement,
sql text not null,
connection text not null,
elapsed_ms integer not null,
executed_at text not null,
tags text not null default '',
favorite integer not null default 0
)`); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := ensureColumns(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func ensureColumns(ctx context.Context, conn *sql.DB) error {
	columns, err := historyColumns(ctx, conn)
	if err != nil {
		return err
	}
	additions := map[string]string{
		"success":   "alter table history add column success integer not null default 1",
		"error":     "alter table history add column error text not null default ''",
		"row_count": "alter table history add column row_count integer not null default 0",
		"driver":    "alter table history add column driver text not null default ''",
	}
	for name, stmt := range additions {
		if columns[name] {
			continue
		}
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func historyColumns(ctx context.Context, conn *sql.DB) (map[string]bool, error) {
	rows, err := conn.QueryContext(ctx, `pragma table_info(history)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}
