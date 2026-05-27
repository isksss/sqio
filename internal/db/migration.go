package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/isksss/sqio/internal/dbdriver"
)

// Migration describes one SQL migration file.
type Migration struct {
	Version        string `json:"version"`
	Name           string `json:"name"`
	Path           string `json:"path"`
	DownPath       string `json:"down_path,omitempty"`
	Applied        bool   `json:"applied"`
	AppliedAt      string `json:"applied_at,omitempty"`
	Checksum       string `json:"checksum,omitempty"`
	StoredChecksum string `json:"stored_checksum,omitempty"`
	Dirty          bool   `json:"dirty,omitempty"`
}

// MigrationResult summarizes an apply run.
type MigrationResult struct {
	Applied []Migration `json:"applied"`
}

type appliedMigration struct {
	AppliedAt string
	Checksum  string
	Dirty     bool
}

// MigrationPlan describes pending apply and rollback candidates.
type MigrationPlan struct {
	Pending  []Migration `json:"pending"`
	Applied  []Migration `json:"applied"`
	Rollback []Migration `json:"rollback"`
}

// MigrationStatus returns SQL files in dir and whether they are already
// recorded in the sqio_migrations table.
func MigrationStatus(ctx context.Context, cfg Config, dir string) ([]Migration, error) {
	migrations, err := readMigrations(dir)
	if err != nil {
		return nil, err
	}
	conn, driver, err := openConnection(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := ensureMigrationTable(ctx, conn, driver); err != nil {
		return nil, err
	}
	applied, err := appliedMigrations(ctx, conn)
	if err != nil {
		return nil, err
	}
	for i := range migrations {
		if state, ok := applied[migrations[i].Version]; ok {
			migrations[i].Applied = true
			migrations[i].AppliedAt = state.AppliedAt
			migrations[i].StoredChecksum = state.Checksum
			migrations[i].Dirty = state.Dirty
		}
	}
	return migrations, nil
}

// ApplyMigrations applies pending SQL migration files in filename order. Each
// file is executed in its own transaction.
func ApplyMigrations(ctx context.Context, cfg Config, dir string, limit int) (MigrationResult, error) {
	migrations, err := MigrationStatus(ctx, cfg, dir)
	if err != nil {
		return MigrationResult{}, err
	}
	if err := validateMigrationState(migrations); err != nil {
		return MigrationResult{}, err
	}
	conn, driver, err := openConnection(ctx, cfg)
	if err != nil {
		return MigrationResult{}, err
	}
	defer conn.Close()
	result := MigrationResult{}
	for _, migration := range migrations {
		if migration.Applied {
			continue
		}
		if limit > 0 && len(result.Applied) >= limit {
			break
		}
		sqlText, err := os.ReadFile(migration.Path)
		if err != nil {
			return MigrationResult{}, err
		}
		if err := applyMigration(ctx, conn, driver, migration, string(sqlText)); err != nil {
			return MigrationResult{}, err
		}
		migration.Applied = true
		migration.AppliedAt = time.Now().UTC().Format(time.RFC3339Nano)
		result.Applied = append(result.Applied, migration)
	}
	return result, nil
}

// PlanMigrations returns pending apply and rollback candidates without changing
// the database.
func PlanMigrations(ctx context.Context, cfg Config, dir string, rollbackLimit int) (MigrationPlan, error) {
	status, err := MigrationStatus(ctx, cfg, dir)
	if err != nil {
		return MigrationPlan{}, err
	}
	if err := validateMigrationState(status); err != nil {
		return MigrationPlan{}, err
	}
	plan := MigrationPlan{}
	for _, migration := range status {
		if migration.Applied {
			plan.Applied = append(plan.Applied, migration)
			continue
		}
		plan.Pending = append(plan.Pending, migration)
	}
	for i := len(plan.Applied) - 1; i >= 0; i-- {
		if rollbackLimit > 0 && len(plan.Rollback) >= rollbackLimit {
			break
		}
		plan.Rollback = append(plan.Rollback, plan.Applied[i])
	}
	return plan, nil
}

// BaselineMigrations records migrations up to version as applied without
// executing their SQL.
func BaselineMigrations(ctx context.Context, cfg Config, dir, version string) (MigrationResult, error) {
	if version == "" {
		return MigrationResult{}, fmt.Errorf("baseline version is required")
	}
	migrations, err := MigrationStatus(ctx, cfg, dir)
	if err != nil {
		return MigrationResult{}, err
	}
	if err := validateMigrationState(migrations); err != nil {
		return MigrationResult{}, err
	}
	conn, driver, err := openConnection(ctx, cfg)
	if err != nil {
		return MigrationResult{}, err
	}
	defer conn.Close()
	result := MigrationResult{}
	for _, migration := range migrations {
		if migration.Applied {
			continue
		}
		if migration.Version > version {
			continue
		}
		if err := recordMigration(ctx, conn, driver, migration, false); err != nil {
			return MigrationResult{}, err
		}
		migration.Applied = true
		migration.AppliedAt = time.Now().UTC().Format(time.RFC3339Nano)
		result.Applied = append(result.Applied, migration)
	}
	return result, nil
}

// RollbackMigrations rolls back the most recently applied migrations that have
// matching .down.sql files.
func RollbackMigrations(ctx context.Context, cfg Config, dir string, limit int) (MigrationResult, error) {
	if limit <= 0 {
		limit = 1
	}
	plan, err := PlanMigrations(ctx, cfg, dir, limit)
	if err != nil {
		return MigrationResult{}, err
	}
	conn, driver, err := openConnection(ctx, cfg)
	if err != nil {
		return MigrationResult{}, err
	}
	defer conn.Close()
	result := MigrationResult{}
	for _, migration := range plan.Rollback {
		if migration.DownPath == "" {
			return MigrationResult{}, fmt.Errorf("down migration not found for %s", migration.Name)
		}
		sqlText, err := os.ReadFile(migration.DownPath)
		if err != nil {
			return MigrationResult{}, err
		}
		if err := rollbackMigration(ctx, conn, driver, migration, string(sqlText)); err != nil {
			return MigrationResult{}, err
		}
		migration.Applied = false
		migration.AppliedAt = ""
		result.Applied = append(result.Applied, migration)
	}
	return result, nil
}

func readMigrations(dir string) ([]Migration, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("migration dir is required")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	migrations := []Migration{}
	downPaths := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		if strings.HasSuffix(name, ".down.sql") {
			downPaths[migrationName(name)] = filepath.Join(dir, name)
			continue
		}
		version := migrationVersion(name)
		migrations = append(migrations, Migration{
			Version: version,
			Name:    migrationName(name),
			Path:    filepath.Join(dir, name),
		})
	}
	for i := range migrations {
		migrations[i].DownPath = downPaths[migrations[i].Name]
		checksum, err := migrationChecksum(migrations[i].Path)
		if err != nil {
			return nil, err
		}
		migrations[i].Checksum = checksum
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})
	return migrations, nil
}

func migrationChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func migrationVersion(name string) string {
	base := migrationName(name)
	for i, r := range base {
		if r == '_' || r == '-' {
			return base[:i]
		}
	}
	return base
}

func migrationName(name string) string {
	base := strings.TrimSuffix(name, ".sql")
	base = strings.TrimSuffix(base, ".up")
	base = strings.TrimSuffix(base, ".down")
	return base
}

func ensureMigrationTable(ctx context.Context, conn *sql.DB, driver string) error {
	if _, err := conn.ExecContext(ctx, `create table if not exists sqio_migrations (
version text primary key,
name text not null,
applied_at text not null,
checksum text not null default '',
dirty integer not null default 0
)`); err != nil {
		return err
	}
	if driver != dbdriver.DriverSQLite && driver != dbdriver.DriverDuckDB {
		return nil
	}
	return ensureMigrationColumns(ctx, conn)
}

func ensureMigrationColumns(ctx context.Context, conn *sql.DB) error {
	columns, err := migrationColumns(ctx, conn)
	if err != nil {
		return err
	}
	alter := map[string]string{
		"checksum": "alter table sqio_migrations add column checksum text not null default ''",
		"dirty":    "alter table sqio_migrations add column dirty integer not null default 0",
	}
	for column, stmt := range alter {
		if !columns[column] {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func migrationColumns(ctx context.Context, conn *sql.DB) (map[string]bool, error) {
	rows, err := conn.QueryContext(ctx, `pragma table_info(sqio_migrations)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name string
		var typ string
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

func appliedMigrations(ctx context.Context, conn *sql.DB) (map[string]appliedMigration, error) {
	rows, err := conn.QueryContext(ctx, `select version, applied_at, checksum, dirty from sqio_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applied := map[string]appliedMigration{}
	for rows.Next() {
		var version string
		var state appliedMigration
		var dirty int
		if err := rows.Scan(&version, &state.AppliedAt, &state.Checksum, &dirty); err != nil {
			return nil, err
		}
		state.Dirty = dirty != 0
		applied[version] = state
	}
	return applied, rows.Err()
}

func applyMigration(ctx context.Context, conn *sql.DB, driver string, migration Migration, sqlText string) error {
	if err := recordMigration(ctx, conn, driver, migration, true); err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		return err
	}
	stmt := fmt.Sprintf(`update sqio_migrations set dirty = 0 where version = %s`, placeholder(driver, 1))
	if _, err := tx.ExecContext(ctx, stmt, migration.Version); err != nil {
		return err
	}
	return tx.Commit()
}

func rollbackMigration(ctx context.Context, conn *sql.DB, driver string, migration Migration, sqlText string) error {
	stmt := fmt.Sprintf(`update sqio_migrations set dirty = 1 where version = %s`, placeholder(driver, 1))
	if _, err := conn.ExecContext(ctx, stmt, migration.Version); err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		return err
	}
	stmt = fmt.Sprintf(`delete from sqio_migrations where version = %s`, placeholder(driver, 1))
	if _, err := tx.ExecContext(ctx, stmt, migration.Version); err != nil {
		return err
	}
	return tx.Commit()
}

func recordMigration(ctx context.Context, conn *sql.DB, driver string, migration Migration, dirty bool) error {
	appliedAt := time.Now().UTC().Format(time.RFC3339Nano)
	dirtyValue := 0
	if dirty {
		dirtyValue = 1
	}
	stmt := fmt.Sprintf(`insert into sqio_migrations (version, name, applied_at, checksum, dirty) values (%s, %s, %s, %s, %s)`, placeholder(driver, 1), placeholder(driver, 2), placeholder(driver, 3), placeholder(driver, 4), placeholder(driver, 5))
	_, err := conn.ExecContext(ctx, stmt, migration.Version, migration.Name, appliedAt, migration.Checksum, dirtyValue)
	return err
}

func validateMigrationState(migrations []Migration) error {
	for _, migration := range migrations {
		if !migration.Applied {
			continue
		}
		if migration.Dirty {
			return fmt.Errorf("migration %s is dirty", migration.Name)
		}
		if migration.StoredChecksum != "" && migration.Checksum != "" && migration.StoredChecksum != migration.Checksum {
			return fmt.Errorf("migration checksum mismatch: %s", migration.Name)
		}
	}
	return nil
}
