// Package history stores executed SQL statements in a local SQLite database.
package history

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
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
	_, err = conn.ExecContext(ctx, `insert into history (sql, connection, elapsed_ms, executed_at, tags, favorite) values (?, ?, ?, ?, ?, ?)`,
		entry.SQL, entry.Connection, entry.ElapsedMS, entry.ExecutedAt.Format(time.RFC3339Nano), entry.Tags, entry.Favorite)
	return err
}

// List returns recent history entries in reverse insertion order. A non-positive
// limit uses the default maximum of one hundred rows.
func (s Store) List(ctx context.Context, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 100
	}
	conn, err := s.open(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, `select id, sql, connection, elapsed_ms, executed_at, tags, favorite from history order by id desc limit ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []Entry{}
	for rows.Next() {
		var entry Entry
		var executedAt string
		if err := rows.Scan(&entry.ID, &entry.SQL, &entry.Connection, &entry.ElapsedMS, &executedAt, &entry.Tags, &entry.Favorite); err != nil {
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
	return conn, nil
}
