package db

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/go-sql-driver/mysql"
	goora "github.com/sijms/go-ora/v2"
)

// Connection is the user-facing connection model used to construct DSNs.
type Connection struct {
	Driver   string
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string
	DSN      string
}

// DSN returns an explicit DSN or builds one from structured connection fields
// using driver-specific defaults.
func DSN(conn Connection) (string, error) {
	if conn.DSN != "" {
		return conn.DSN, nil
	}
	switch conn.Driver {
	case "sqlite", "sqlite3":
		if conn.Database == "" {
			return "", fmt.Errorf("sqlite requires database path")
		}
		return conn.Database, nil
	case "duckdb":
		if conn.Database == "" {
			return "", fmt.Errorf("duckdb requires database path")
		}
		return conn.Database, nil
	case "postgres", "postgresql", "pgx", "cockroach", "cockroachdb":
		if conn.Host == "" {
			conn.Host = "localhost"
		}
		if conn.Port == 0 {
			conn.Port = 5432
		}
		if conn.SSLMode == "" {
			conn.SSLMode = "disable"
		}
		u := url.URL{
			Scheme: "postgres",
			Host:   fmt.Sprintf("%s:%d", conn.Host, conn.Port),
			Path:   conn.Database,
		}
		if conn.User != "" {
			u.User = url.UserPassword(conn.User, conn.Password)
		}
		q := u.Query()
		q.Set("sslmode", conn.SSLMode)
		u.RawQuery = q.Encode()
		return u.String(), nil
	case "mysql", "mariadb", "tidb":
		if conn.Host == "" {
			conn.Host = "localhost"
		}
		if conn.Port == 0 {
			conn.Port = 3306
		}
		cfg := mysql.NewConfig()
		cfg.User = conn.User
		cfg.Passwd = conn.Password
		cfg.Net = "tcp"
		cfg.Addr = net.JoinHostPort(conn.Host, strconv.Itoa(conn.Port))
		cfg.DBName = conn.Database
		return cfg.FormatDSN(), nil
	case "sqlserver", "mssql":
		if conn.Host == "" {
			conn.Host = "localhost"
		}
		if conn.Port == 0 {
			conn.Port = 1433
		}
		u := url.URL{
			Scheme: "sqlserver",
			Host:   net.JoinHostPort(conn.Host, strconv.Itoa(conn.Port)),
		}
		if conn.User != "" {
			u.User = url.UserPassword(conn.User, conn.Password)
		}
		q := u.Query()
		if conn.Database != "" {
			q.Set("database", conn.Database)
		}
		u.RawQuery = q.Encode()
		return u.String(), nil
	case "oracle":
		if conn.Host == "" {
			conn.Host = "localhost"
		}
		if conn.Port == 0 {
			conn.Port = 1521
		}
		if conn.Database == "" {
			return "", fmt.Errorf("oracle requires service name")
		}
		return goora.BuildUrl(conn.Host, conn.Port, conn.Database, conn.User, conn.Password, nil), nil
	case "clickhouse", "ch":
		if conn.Host == "" {
			conn.Host = "localhost"
		}
		if conn.Port == 0 {
			conn.Port = 9000
		}
		u := url.URL{
			Scheme: "clickhouse",
			Host:   net.JoinHostPort(conn.Host, strconv.Itoa(conn.Port)),
			Path:   conn.Database,
		}
		if conn.User != "" {
			u.User = url.UserPassword(conn.User, conn.Password)
		}
		return u.String(), nil
	default:
		return "", fmt.Errorf("unsupported driver: %s", conn.Driver)
	}
}
