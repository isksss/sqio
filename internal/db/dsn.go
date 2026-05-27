package db

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/go-sql-driver/mysql"
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
	case "postgres", "postgresql", "pgx":
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
	case "mysql":
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
	default:
		return "", fmt.Errorf("unsupported driver: %s", conn.Driver)
	}
}
