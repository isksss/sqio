package cli

import (
	"context"

	"github.com/isksss/sqio/internal/config"
	"github.com/isksss/sqio/internal/db"
	"github.com/isksss/sqio/internal/secret"
	"github.com/isksss/sqio/internal/tunnel"
)

type connectionOptions struct {
	conn          string
	driver        string
	dsn           string
	database      string
	host          string
	port          int
	user          string
	password      string
	sslMode       string
	readonly      bool
	ageIdentity   string
	sshTunnel     bool
	sshHost       string
	sshPort       int
	sshUser       string
	sshPassword   string
	sshPrivateKey string
}

func resolveConnection(cfg config.Config, opts execOptions) (string, string, error) {
	driver, dsn, cleanup, err := prepareConnection(context.Background(), cfg, opts.connectionOptions)
	if cleanup != nil {
		cleanup()
	}
	return driver, dsn, err
}

func resolveConnectionOptions(cfg config.Config, opts connectionOptions) (string, string, error) {
	driver, dsn, cleanup, err := prepareConnection(context.Background(), cfg, opts)
	if cleanup != nil {
		cleanup()
	}
	return driver, dsn, err
}

func prepareConnection(ctx context.Context, cfg config.Config, opts connectionOptions) (string, string, func(), error) {
	conn := db.Connection{
		Driver:   opts.driver,
		Host:     opts.host,
		Port:     opts.port,
		Database: opts.database,
		User:     opts.user,
		Password: opts.password,
		SSLMode:  opts.sslMode,
		DSN:      opts.dsn,
	}
	tunnelConfig := tunnel.Config{
		Enabled:    opts.sshTunnel,
		Host:       opts.sshHost,
		Port:       opts.sshPort,
		User:       opts.sshUser,
		Password:   opts.sshPassword,
		PrivateKey: opts.sshPrivateKey,
	}
	if opts.conn != "" {
		configConn, err := cfg.Connection(opts.conn)
		if err != nil {
			return "", "", nil, &CommandError{Type: "connection", Message: err.Error(), Code: ExitConnection}
		}
		conn = db.Connection{
			Driver:   configConn.Driver,
			Host:     configConn.Host,
			Port:     configConn.Port,
			Database: configConn.Database,
			User:     configConn.User,
			Password: configConn.Password,
			SSLMode:  configConn.SSLMode,
			DSN:      configConn.DSN,
		}
		tunnelConfig = tunnel.Config{
			Enabled:    configConn.SSHTunnel.Enabled || opts.sshTunnel,
			Host:       firstNonEmpty(opts.sshHost, configConn.SSHTunnel.Host),
			Port:       firstNonZero(opts.sshPort, configConn.SSHTunnel.Port),
			User:       firstNonEmpty(opts.sshUser, configConn.SSHTunnel.User),
			Password:   firstNonEmpty(opts.sshPassword, configConn.SSHTunnel.Password),
			PrivateKey: firstNonEmpty(opts.sshPrivateKey, configConn.SSHTunnel.PrivateKey),
		}
	}
	if conn.Password != "" && opts.ageIdentity != "" {
		decrypted, err := secret.DecryptAge(conn.Password, opts.ageIdentity)
		if err != nil {
			return "", "", nil, &CommandError{Type: "connection", Message: err.Error(), Code: ExitConnection}
		}
		conn.Password = decrypted
	}
	if conn.Driver == "" && conn.DSN == "" {
		return "", "", nil, nil
	}
	cleanup := func() {}
	if tunnelConfig.Enabled {
		if conn.DSN != "" {
			return "", "", nil, &CommandError{Type: "connection", Message: "ssh tunnel requires host/port connection fields instead of dsn", Code: ExitConnection}
		}
		tunnelConfig.RemoteHost = firstNonEmpty(conn.Host, "localhost")
		tunnelConfig.RemotePort = firstNonZero(conn.Port, defaultPort(conn.Driver))
		sshTunnel, err := tunnel.Start(ctx, tunnelConfig)
		if err != nil {
			return "", "", nil, &CommandError{Type: "connection", Message: err.Error(), Code: ExitConnection}
		}
		if sshTunnel != nil {
			cleanup = func() { _ = sshTunnel.Close() }
			conn.Host = sshTunnel.LocalHost()
			conn.Port = sshTunnel.LocalPort()
		}
	}
	dsn, err := db.DSN(conn)
	if err != nil {
		cleanup()
		return "", "", nil, &CommandError{Type: "connection", Message: err.Error(), Code: ExitConnection}
	}
	return conn.Driver, dsn, cleanup, nil
}

func readonlyEnabled(cfg config.Config, opts connectionOptions) bool {
	if opts.readonly {
		return true
	}
	if opts.conn == "" {
		return false
	}
	configConn, err := cfg.Connection(opts.conn)
	return err == nil && configConn.Readonly
}

func addConnectionFlags(flags interface {
	StringVar(*string, string, string, string)
	IntVar(*int, string, int, string)
	BoolVar(*bool, string, bool, string)
}, opts *connectionOptions) {
	flags.StringVar(&opts.conn, "conn", "", "connection name")
	flags.StringVar(&opts.driver, "driver", "", "database driver")
	flags.StringVar(&opts.dsn, "dsn", "", "database DSN")
	flags.StringVar(&opts.database, "database", "", "database name or sqlite path")
	flags.StringVar(&opts.host, "host", "", "database host")
	flags.IntVar(&opts.port, "port", 0, "database port")
	flags.StringVar(&opts.user, "user", "", "database user")
	flags.StringVar(&opts.password, "password", "", "database password")
	flags.StringVar(&opts.sslMode, "sslmode", "", "database SSL mode")
	flags.BoolVar(&opts.readonly, "readonly", false, "enforce readonly safety checks")
	flags.StringVar(&opts.ageIdentity, "age-identity", "", "age identity file")
	flags.BoolVar(&opts.sshTunnel, "ssh-tunnel", false, "enable SSH tunnel")
	flags.StringVar(&opts.sshHost, "ssh-host", "", "SSH tunnel host")
	flags.IntVar(&opts.sshPort, "ssh-port", 0, "SSH tunnel port")
	flags.StringVar(&opts.sshUser, "ssh-user", "", "SSH tunnel user")
	flags.StringVar(&opts.sshPassword, "ssh-password", "", "SSH tunnel password")
	flags.StringVar(&opts.sshPrivateKey, "ssh-private-key", "", "SSH tunnel private key")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func defaultPort(driver string) int {
	switch driver {
	case "postgres", "postgresql", "pgx":
		return 5432
	case "mysql":
		return 3306
	default:
		return 0
	}
}
