package cli

import (
	"context"
	"time"

	"github.com/isksss/sqio/internal/config"
	"github.com/isksss/sqio/internal/db"
	"github.com/isksss/sqio/internal/dbdriver"
	"github.com/isksss/sqio/internal/secret"
	"github.com/isksss/sqio/internal/tunnel"
)

// connectionOptions contains flags that identify a database connection or SSH
// tunnel.
type connectionOptions struct {
	conn                 string
	driver               string
	dsn                  string
	database             string
	host                 string
	port                 int
	user                 string
	password             string
	sslMode              string
	readonly             bool
	ageIdentity          string
	sshTunnel            bool
	sshHost              string
	sshPort              int
	sshUser              string
	sshPassword          string
	sshPrivateKey        string
	sshKnownHosts        string
	sshKeepAlive         string
	sshReconnect         bool
	sshReconnectAttempts int
	sshJumpHost          string
	sshJumpPort          int
	sshJumpUser          string
	sshJumpPassword      string
	sshJumpPrivateKey    string
	sshJumpKnownHosts    string
}

// resolveConnection resolves execOptions into a driver and DSN for tests and
// compatibility helpers.
func resolveConnection(cfg config.Config, opts execOptions) (string, string, error) {
	driver, dsn, cleanup, err := prepareConnection(context.Background(), cfg, opts.connectionOptions)
	if cleanup != nil {
		cleanup()
	}
	return driver, dsn, err
}

// resolveConnectionOptions resolves connectionOptions into a driver and DSN.
func resolveConnectionOptions(cfg config.Config, opts connectionOptions) (string, string, error) {
	driver, dsn, cleanup, err := prepareConnection(context.Background(), cfg, opts)
	if cleanup != nil {
		cleanup()
	}
	return driver, dsn, err
}

// prepareConnection merges CLI flags with named configuration, decrypts
// password values when requested, starts an optional SSH tunnel, and returns a
// cleanup function for transient resources.
func prepareConnection(ctx context.Context, cfg config.Config, opts connectionOptions) (string, string, func(), error) {
	passwordEncrypted := false
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
		Enabled:           opts.sshTunnel,
		Host:              opts.sshHost,
		Port:              opts.sshPort,
		User:              opts.sshUser,
		Password:          opts.sshPassword,
		PrivateKey:        opts.sshPrivateKey,
		KnownHosts:        opts.sshKnownHosts,
		Reconnect:         opts.sshReconnect,
		ReconnectAttempts: opts.sshReconnectAttempts,
		JumpHost:          opts.sshJumpHost,
		JumpPort:          opts.sshJumpPort,
		JumpUser:          opts.sshJumpUser,
		JumpPassword:      opts.sshJumpPassword,
		JumpPrivateKey:    opts.sshJumpPrivateKey,
		JumpKnownHosts:    opts.sshJumpKnownHosts,
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
		passwordEncrypted = configConn.PasswordEncrypted
		tunnelConfig = tunnel.Config{
			Enabled:           configConn.SSHTunnel.Enabled || opts.sshTunnel,
			Host:              firstNonEmpty(opts.sshHost, configConn.SSHTunnel.Host),
			Port:              firstNonZero(opts.sshPort, configConn.SSHTunnel.Port),
			User:              firstNonEmpty(opts.sshUser, configConn.SSHTunnel.User),
			Password:          firstNonEmpty(opts.sshPassword, configConn.SSHTunnel.Password),
			PrivateKey:        firstNonEmpty(opts.sshPrivateKey, configConn.SSHTunnel.PrivateKey),
			KnownHosts:        firstNonEmpty(opts.sshKnownHosts, configConn.SSHTunnel.KnownHosts),
			Reconnect:         opts.sshReconnect || configConn.SSHTunnel.Reconnect,
			ReconnectAttempts: firstNonZero(opts.sshReconnectAttempts, configConn.SSHTunnel.ReconnectAttempts),
			JumpHost:          firstNonEmpty(opts.sshJumpHost, configConn.SSHTunnel.JumpHost),
			JumpPort:          firstNonZero(opts.sshJumpPort, configConn.SSHTunnel.JumpPort),
			JumpUser:          firstNonEmpty(opts.sshJumpUser, configConn.SSHTunnel.JumpUser),
			JumpPassword:      firstNonEmpty(opts.sshJumpPassword, configConn.SSHTunnel.JumpPassword),
			JumpPrivateKey:    firstNonEmpty(opts.sshJumpPrivateKey, configConn.SSHTunnel.JumpPrivateKey),
			JumpKnownHosts:    firstNonEmpty(opts.sshJumpKnownHosts, configConn.SSHTunnel.JumpKnownHosts),
		}
		if keepAlive := firstNonEmpty(opts.sshKeepAlive, configConn.SSHTunnel.KeepAlive); keepAlive != "" {
			interval, err := time.ParseDuration(keepAlive)
			if err != nil {
				return "", "", nil, &CommandError{Type: "connection", Message: err.Error(), Code: ExitConnection}
			}
			tunnelConfig.KeepAliveInterval = interval
		}
	} else if opts.sshKeepAlive != "" {
		interval, err := time.ParseDuration(opts.sshKeepAlive)
		if err != nil {
			return "", "", nil, &CommandError{Type: "connection", Message: err.Error(), Code: ExitConnection}
		}
		tunnelConfig.KeepAliveInterval = interval
	}
	if passwordEncrypted && opts.ageIdentity == "" {
		return "", "", nil, &CommandError{Type: "connection", Message: "age identity is required for encrypted password", Code: ExitConnection}
	}
	if passwordEncrypted && conn.Password != "" {
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

// readonlyEnabled reports whether readonly safety checks should be enforced for
// the selected connection.
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

// addConnectionFlags registers shared database and SSH connection flags on a
// command.
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
	flags.StringVar(&opts.sshKnownHosts, "ssh-known-hosts", "", "SSH known_hosts file")
	flags.StringVar(&opts.sshKeepAlive, "ssh-keepalive", "", "SSH keepalive interval")
	flags.BoolVar(&opts.sshReconnect, "ssh-reconnect", false, "reconnect SSH tunnel on remote dial failure")
	flags.IntVar(&opts.sshReconnectAttempts, "ssh-reconnect-attempts", 0, "SSH reconnect attempts")
	flags.StringVar(&opts.sshJumpHost, "ssh-jump-host", "", "SSH jump host")
	flags.IntVar(&opts.sshJumpPort, "ssh-jump-port", 0, "SSH jump port")
	flags.StringVar(&opts.sshJumpUser, "ssh-jump-user", "", "SSH jump user")
	flags.StringVar(&opts.sshJumpPassword, "ssh-jump-password", "", "SSH jump password")
	flags.StringVar(&opts.sshJumpPrivateKey, "ssh-jump-private-key", "", "SSH jump private key")
	flags.StringVar(&opts.sshJumpKnownHosts, "ssh-jump-known-hosts", "", "SSH jump known_hosts file")
}

// firstNonEmpty returns the first non-empty string in values.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// firstNonZero returns the first non-zero integer in values.
func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

// defaultPort returns the conventional TCP port for known database drivers.
func defaultPort(driver string) int {
	return dbdriver.DefaultPort(driver)
}
