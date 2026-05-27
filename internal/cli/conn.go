package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/isksss/sqio/internal/config"
	"github.com/isksss/sqio/internal/db"
	"github.com/spf13/cobra"
)

type connListEntry struct {
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	Database  string `json:"database,omitempty"`
	User      string `json:"user,omitempty"`
	Readonly  bool   `json:"readonly"`
	SSHTunnel bool   `json:"ssh_tunnel"`
}

type connTestResult struct {
	Connection string `json:"connection"`
	Driver     string `json:"driver"`
	OK         bool   `json:"ok"`
	ElapsedMS  int64  `json:"elapsed_ms"`
}

// newConnCommand creates connection inspection and diagnostics commands.
func newConnCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "conn", Short: "connection operations"}
	cmd.AddCommand(newConnListCommand(), newConnTestCommand(), newConnAddCommand(), newConnRemoveCommand())
	return cmd
}

func newConnListCommand() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list configured connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			entries := make([]connListEntry, 0, len(cfg.Connections))
			for _, conn := range cfg.Connections {
				entries = append(entries, connListEntry{
					Name:      conn.Name,
					Driver:    conn.Driver,
					Host:      conn.Host,
					Port:      conn.Port,
					Database:  conn.Database,
					User:      conn.User,
					Readonly:  conn.Readonly,
					SSHTunnel: conn.SSHTunnel.Enabled,
				})
			}
			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			case "", "table":
				for _, entry := range entries {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", entry.Name, entry.Driver, entry.Database, readonlyLabel(entry.Readonly))
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported connection format: " + format, Code: ExitInternal}
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	return cmd
}

func newConnTestCommand() *cobra.Command {
	var opts connectionOptions
	var format string
	var timeoutText string
	cmd := &cobra.Command{
		Use:   "test",
		Short: "test a database connection",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			timeout, err := connTestTimeout(cfg, timeoutText)
			if err != nil {
				return &CommandError{Type: "config", Message: err.Error(), Code: ExitInternal}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			driver, dsn, cleanup, err := prepareConnection(ctx, cfg, opts)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			if driver == "" && dsn == "" {
				return &CommandError{Type: "connection", Message: "connection is required", Code: ExitConnection}
			}
			started := time.Now()
			conn, normalizedDriver, err := db.Open(ctx, db.Config{Driver: driver, DSN: dsn})
			if err != nil {
				return &CommandError{Type: "connection", Message: err.Error(), Code: ExitConnection}
			}
			defer conn.Close()
			result := connTestResult{
				Connection: connTestLabel(opts),
				Driver:     normalizedDriver,
				OK:         true,
				ElapsedMS:  time.Since(started).Milliseconds(),
			}
			switch format {
			case "json":
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			case "", "table":
				if !global.quiet {
					fmt.Fprintf(cmd.OutOrStdout(), "ok\t%s\t%s\t%dms\n", result.Connection, result.Driver, result.ElapsedMS)
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported connection format: " + format, Code: ExitInternal}
			}
		},
	}
	addConnectionFlags(cmd.Flags(), &opts)
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	cmd.Flags().StringVar(&timeoutText, "timeout", "", "connection timeout")
	return cmd
}

func newConnAddCommand() *cobra.Command {
	var conn config.Connection
	cmd := &cobra.Command{
		Use:   "add NAME",
		Short: "add or replace a configured connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn.Name = args[0]
			issues := config.Validate(config.Config{Query: config.Default().Query, Connections: []config.Connection{conn}})
			if len(issues) > 0 {
				return &CommandError{Type: "config", Message: issues[0].Path + ": " + issues[0].Message, Code: ExitInternal}
			}
			if err := config.UpsertConnection(configWritePath(), conn); err != nil {
				return &CommandError{Type: "config", Message: err.Error(), Code: ExitInternal}
			}
			if !global.quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "saved\t%s\n", conn.Name)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&conn.Driver, "driver", "", "database driver")
	cmd.Flags().StringVar(&conn.Host, "host", "", "database host")
	cmd.Flags().IntVar(&conn.Port, "port", 0, "database port")
	cmd.Flags().StringVar(&conn.Database, "database", "", "database name or sqlite path")
	cmd.Flags().StringVar(&conn.User, "user", "", "database user")
	cmd.Flags().StringVar(&conn.Password, "password", "", "database password or env:NAME reference")
	cmd.Flags().BoolVar(&conn.PasswordEncrypted, "password-encrypted", false, "password is age-encrypted")
	cmd.Flags().BoolVar(&conn.Readonly, "readonly", false, "treat connection as readonly")
	cmd.Flags().StringVar(&conn.SSLMode, "sslmode", "", "postgres sslmode")
	cmd.Flags().StringVar(&conn.DSN, "dsn", "", "database DSN")
	cmd.Flags().BoolVar(&conn.SSHTunnel.Enabled, "ssh-tunnel", false, "enable SSH tunnel")
	cmd.Flags().StringVar(&conn.SSHTunnel.Host, "ssh-host", "", "SSH host")
	cmd.Flags().IntVar(&conn.SSHTunnel.Port, "ssh-port", 22, "SSH port")
	cmd.Flags().StringVar(&conn.SSHTunnel.User, "ssh-user", "", "SSH user")
	cmd.Flags().StringVar(&conn.SSHTunnel.Password, "ssh-password", "", "SSH password or env:NAME reference")
	cmd.Flags().StringVar(&conn.SSHTunnel.PrivateKey, "ssh-private-key", "", "SSH private key path")
	cmd.Flags().StringVar(&conn.SSHTunnel.KnownHosts, "ssh-known-hosts", "", "SSH known_hosts path")
	cmd.Flags().StringVar(&conn.SSHTunnel.KeepAlive, "ssh-keepalive", "", "SSH keepalive interval")
	cmd.Flags().BoolVar(&conn.SSHTunnel.Reconnect, "ssh-reconnect", false, "reconnect SSH tunnel on remote dial failure")
	cmd.Flags().IntVar(&conn.SSHTunnel.ReconnectAttempts, "ssh-reconnect-attempts", 0, "SSH reconnect attempts")
	cmd.Flags().StringVar(&conn.SSHTunnel.JumpHost, "ssh-jump-host", "", "SSH jump host")
	cmd.Flags().IntVar(&conn.SSHTunnel.JumpPort, "ssh-jump-port", 0, "SSH jump port")
	cmd.Flags().StringVar(&conn.SSHTunnel.JumpUser, "ssh-jump-user", "", "SSH jump user")
	cmd.Flags().StringVar(&conn.SSHTunnel.JumpPassword, "ssh-jump-password", "", "SSH jump password or env:NAME reference")
	cmd.Flags().StringVar(&conn.SSHTunnel.JumpPrivateKey, "ssh-jump-private-key", "", "SSH jump private key path")
	cmd.Flags().StringVar(&conn.SSHTunnel.JumpKnownHosts, "ssh-jump-known-hosts", "", "SSH jump known_hosts path")
	return cmd
}

func newConnRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove NAME",
		Aliases: []string{"rm"},
		Short:   "remove a configured connection",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RemoveConnection(configWritePath(), args[0]); err != nil {
				return &CommandError{Type: "config", Message: err.Error(), Code: ExitInternal}
			}
			if !global.quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "removed\t%s\n", args[0])
			}
			return nil
		},
	}
	return cmd
}

func connTestTimeout(cfg config.Config, timeoutText string) (time.Duration, error) {
	if timeoutText != "" {
		return time.ParseDuration(timeoutText)
	}
	return config.TimeoutDuration(cfg)
}

func connTestLabel(opts connectionOptions) string {
	if opts.conn != "" {
		return opts.conn
	}
	return "direct"
}

func readonlyLabel(readonly bool) string {
	if readonly {
		return "readonly"
	}
	return "readwrite"
}

func configWritePath() string {
	if global.configPath != "" {
		return global.configPath
	}
	return config.DefaultPath()
}
