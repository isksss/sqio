package db

import (
	"fmt"
	"net/url"
)

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
		auth := conn.User
		if conn.Password != "" {
			auth += ":" + conn.Password
		}
		if auth != "" {
			auth += "@"
		}
		return fmt.Sprintf("%stcp(%s:%d)/%s", auth, conn.Host, conn.Port, conn.Database), nil
	default:
		return "", fmt.Errorf("unsupported driver: %s", conn.Driver)
	}
}
