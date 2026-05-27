// Package dbdriver centralizes database driver names, aliases, and small
// classification helpers used across sqio.
package dbdriver

import "strings"

const (
	DriverSQLite      = "sqlite"
	DriverSQLite3     = "sqlite3"
	DriverDuckDB      = "duckdb"
	DriverPostgres    = "postgres"
	DriverPostgreSQL  = "postgresql"
	DriverPGX         = "pgx"
	DriverCockroach   = "cockroach"
	DriverCockroachDB = "cockroachdb"
	DriverMySQL       = "mysql"
	DriverMariaDB     = "mariadb"
	DriverTiDB        = "tidb"
	DriverSQLServer   = "sqlserver"
	DriverMSSQL       = "mssql"
	DriverOracle      = "oracle"
	DriverClickHouse  = "clickhouse"
	DriverCH          = "ch"
)

// Normalize maps user-facing driver aliases onto database/sql driver names.
func Normalize(driver string) (string, bool) {
	switch strings.ToLower(driver) {
	case DriverSQLite, DriverSQLite3:
		return DriverSQLite, true
	case DriverDuckDB:
		return DriverDuckDB, true
	case DriverPostgres, DriverPostgreSQL, DriverPGX, DriverCockroach, DriverCockroachDB:
		return DriverPGX, true
	case DriverMySQL, DriverMariaDB, DriverTiDB:
		return DriverMySQL, true
	case DriverSQLServer, DriverMSSQL:
		return DriverSQLServer, true
	case DriverOracle:
		return DriverOracle, true
	case DriverClickHouse, DriverCH:
		return DriverClickHouse, true
	default:
		return "", false
	}
}

// Supported reports whether driver is one of sqio's accepted driver names or
// aliases.
func Supported(driver string) bool {
	_, ok := Normalize(driver)
	return ok
}

func IsSQLite(driver string) bool {
	normalized, ok := Normalize(driver)
	return ok && normalized == DriverSQLite
}

func IsPostgresFamily(driver string) bool {
	normalized, ok := Normalize(driver)
	return ok && normalized == DriverPGX
}

func IsMySQLFamily(driver string) bool {
	normalized, ok := Normalize(driver)
	return ok && normalized == DriverMySQL
}

func IsSQLServer(driver string) bool {
	normalized, ok := Normalize(driver)
	return ok && normalized == DriverSQLServer
}

func IsClickHouse(driver string) bool {
	normalized, ok := Normalize(driver)
	return ok && normalized == DriverClickHouse
}

// FamilyName returns the user-facing family name used in diagnostics.
func FamilyName(driver string) string {
	normalized, ok := Normalize(driver)
	if !ok {
		return strings.ToLower(driver)
	}
	switch normalized {
	case DriverPGX:
		return DriverPostgres
	default:
		return normalized
	}
}

// DefaultPort returns the conventional TCP port for networked database drivers.
func DefaultPort(driver string) int {
	switch {
	case IsPostgresFamily(driver):
		return 5432
	case IsMySQLFamily(driver):
		return 3306
	case IsSQLServer(driver):
		return 1433
	case strings.EqualFold(driver, DriverOracle):
		return 1521
	case IsClickHouse(driver):
		return 9000
	default:
		return 0
	}
}
