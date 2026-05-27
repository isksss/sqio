// Package config loads sqio configuration from defaults, TOML files, and
// environment variables.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/isksss/sqio/internal/dbdriver"
	"github.com/isksss/sqio/internal/secret"
)

// Config is the top-level sqio configuration model.
type Config struct {
	Theme       string       `toml:"theme"`
	Editor      string       `toml:"editor"`
	Query       QueryConfig  `toml:"query"`
	Formatter   FormatConfig `toml:"formatter"`
	Lint        LintConfig   `toml:"lint"`
	Connections []Connection `toml:"connections"`
}

// QueryConfig controls SQL execution defaults.
type QueryConfig struct {
	Timeout string `toml:"timeout"`
	MaxRows int    `toml:"max_rows"`
	Format  string `toml:"format"`
}

// FormatConfig controls SQL formatter defaults.
type FormatConfig struct {
	Dialect        string `toml:"dialect"`
	KeywordCase    string `toml:"keyword_case"`
	IdentifierCase string `toml:"identifier_case"`
	Indent         int    `toml:"indent"`
	LineWidth      int    `toml:"line_width"`
}

// LintConfig controls SQL lint rule filtering.
type LintConfig struct {
	Level   string   `toml:"level"`
	Enable  []string `toml:"enable"`
	Disable []string `toml:"disable"`
}

// Connection describes a named database connection from configuration.
type Connection struct {
	Name              string    `toml:"name"`
	Driver            string    `toml:"driver"`
	Host              string    `toml:"host"`
	Port              int       `toml:"port"`
	Database          string    `toml:"database"`
	User              string    `toml:"user"`
	Password          string    `toml:"password"`
	PasswordEncrypted bool      `toml:"password_encrypted"`
	Readonly          bool      `toml:"readonly"`
	SSLMode           string    `toml:"sslmode"`
	DSN               string    `toml:"dsn"`
	SSHTunnel         SSHTunnel `toml:"ssh_tunnel"`
}

// ValidationIssue describes a configuration problem that can be reported
// without opening database connections.
type ValidationIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// SSHTunnel describes optional SSH forwarding settings for a connection.
type SSHTunnel struct {
	Enabled           bool   `toml:"enabled"`
	Host              string `toml:"host"`
	Port              int    `toml:"port"`
	User              string `toml:"user"`
	Password          string `toml:"password"`
	PrivateKey        string `toml:"private_key"`
	KnownHosts        string `toml:"known_hosts"`
	KeepAlive         string `toml:"keepalive"`
	Reconnect         bool   `toml:"reconnect"`
	ReconnectAttempts int    `toml:"reconnect_attempts"`
	JumpHost          string `toml:"jump_host"`
	JumpPort          int    `toml:"jump_port"`
	JumpUser          string `toml:"jump_user"`
	JumpPassword      string `toml:"jump_password"`
	JumpPrivateKey    string `toml:"jump_private_key"`
	JumpKnownHosts    string `toml:"jump_known_hosts"`
}

// Default returns sqio's built-in configuration before file and environment
// overrides are applied.
func Default() Config {
	return Config{
		Theme:  "dark",
		Editor: "vi",
		Query: QueryConfig{
			Timeout: "30s",
			MaxRows: 1000,
			Format:  "table",
		},
		Formatter: FormatConfig{
			Dialect:        "postgres",
			KeywordCase:    "upper",
			IdentifierCase: "lower",
			Indent:         2,
			LineWidth:      100,
		},
		Lint: LintConfig{
			Level: "warning",
		},
	}
}

// Load reads configuration from path when provided, otherwise from the global
// config and the nearest local sqio.toml, then applies supported environment
// variable overrides.
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, &cfg); err != nil {
				return cfg, err
			}
		} else if !os.IsNotExist(err) {
			return cfg, err
		}
		applyEnv(&cfg)
		if err := expandConnectionEnv(&cfg); err != nil {
			return cfg, err
		}
		return cfg, nil
	}
	if err := mergeFile(&cfg, DefaultPath()); err != nil {
		return cfg, err
	}
	localPath, err := FindLocalPath("")
	if err != nil {
		return cfg, err
	}
	if localPath != "" {
		if err := mergeFile(&cfg, localPath); err != nil {
			return cfg, err
		}
	}
	applyEnv(&cfg)
	if err := expandConnectionEnv(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Save writes cfg as TOML, creating parent directories when needed.
func Save(path string, cfg Config) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

// UpsertConnection adds or replaces a connection in path.
func UpsertConnection(path string, conn Connection) error {
	cfg, err := loadMutable(path)
	if err != nil {
		return err
	}
	cfg.Connections = mergeConnections(cfg.Connections, []Connection{conn})
	return Save(path, cfg)
}

// RemoveConnection removes a named connection from path.
func RemoveConnection(path, name string) error {
	cfg, err := loadMutable(path)
	if err != nil {
		return err
	}
	next := cfg.Connections[:0]
	removed := false
	for _, conn := range cfg.Connections {
		if conn.Name == name {
			removed = true
			continue
		}
		next = append(next, conn)
	}
	if !removed {
		return fmt.Errorf("connection not found: %s", name)
	}
	cfg.Connections = next
	return Save(path, cfg)
}

// TimeoutDuration parses the query timeout string into a time.Duration.
func TimeoutDuration(cfg Config) (time.Duration, error) {
	return time.ParseDuration(cfg.Query.Timeout)
}

func loadMutable(path string) (Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	cfg := Default()
	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return cfg, err
		}
	} else if !os.IsNotExist(err) {
		return cfg, err
	}
	return cfg, nil
}

// Validate checks static configuration fields without opening database
// connections or decrypting secrets.
func Validate(cfg Config) []ValidationIssue {
	issues := []ValidationIssue{}
	if _, err := time.ParseDuration(cfg.Query.Timeout); err != nil {
		issues = append(issues, ValidationIssue{Path: "query.timeout", Message: err.Error()})
	}
	if cfg.Query.MaxRows < 0 {
		issues = append(issues, ValidationIssue{Path: "query.max_rows", Message: "must be greater than or equal to 0"})
	}
	if !supportedQueryFormat(cfg.Query.Format) {
		issues = append(issues, ValidationIssue{Path: "query.format", Message: "unsupported format: " + cfg.Query.Format})
	}
	seen := map[string]bool{}
	for i, conn := range cfg.Connections {
		path := fmt.Sprintf("connections[%d]", i)
		if conn.Name == "" {
			issues = append(issues, ValidationIssue{Path: path + ".name", Message: "connection name is required"})
		} else if seen[conn.Name] {
			issues = append(issues, ValidationIssue{Path: path + ".name", Message: "duplicate connection name: " + conn.Name})
		}
		if conn.Name != "" {
			seen[conn.Name] = true
		}
		if conn.Driver == "" {
			issues = append(issues, ValidationIssue{Path: path + ".driver", Message: "driver is required"})
		} else if !supportedDriver(conn.Driver) {
			issues = append(issues, ValidationIssue{Path: path + ".driver", Message: "unsupported driver: " + conn.Driver})
		}
		if dbdriver.IsSQLite(conn.Driver) && conn.DSN == "" && conn.Database == "" {
			issues = append(issues, ValidationIssue{Path: path + ".database", Message: "sqlite requires database or dsn"})
		}
		if conn.SSHTunnel.Enabled {
			if conn.DSN != "" {
				issues = append(issues, ValidationIssue{Path: path + ".ssh_tunnel", Message: "ssh tunnel requires host/port fields instead of dsn"})
			}
			if conn.SSHTunnel.Host == "" {
				issues = append(issues, ValidationIssue{Path: path + ".ssh_tunnel.host", Message: "ssh host is required"})
			}
			if conn.SSHTunnel.User == "" {
				issues = append(issues, ValidationIssue{Path: path + ".ssh_tunnel.user", Message: "ssh user is required"})
			}
			if conn.SSHTunnel.Password == "" && conn.SSHTunnel.PrivateKey == "" {
				issues = append(issues, ValidationIssue{Path: path + ".ssh_tunnel", Message: "ssh password or private_key is required"})
			}
			if conn.SSHTunnel.KeepAlive != "" {
				if _, err := time.ParseDuration(conn.SSHTunnel.KeepAlive); err != nil {
					issues = append(issues, ValidationIssue{Path: path + ".ssh_tunnel.keepalive", Message: err.Error()})
				}
			}
			if conn.SSHTunnel.ReconnectAttempts < 0 {
				issues = append(issues, ValidationIssue{Path: path + ".ssh_tunnel.reconnect_attempts", Message: "must be greater than or equal to 0"})
			}
			if conn.SSHTunnel.JumpHost != "" {
				if conn.SSHTunnel.JumpUser == "" && conn.SSHTunnel.User == "" {
					issues = append(issues, ValidationIssue{Path: path + ".ssh_tunnel.jump_user", Message: "ssh jump user is required"})
				}
				if conn.SSHTunnel.JumpPassword == "" && conn.SSHTunnel.JumpPrivateKey == "" && conn.SSHTunnel.Password == "" && conn.SSHTunnel.PrivateKey == "" {
					issues = append(issues, ValidationIssue{Path: path + ".ssh_tunnel", Message: "ssh jump password or private_key is required"})
				}
			}
		}
	}
	return issues
}

func supportedQueryFormat(format string) bool {
	switch strings.ToLower(format) {
	case "", "table", "json", "jsonl", "csv", "tsv", "markdown", "yaml":
		return true
	default:
		return false
	}
}

func supportedDriver(driver string) bool {
	return dbdriver.Supported(driver)
}

// DefaultPath returns the per-user global configuration path.
func DefaultPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "sqio", "config.toml")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "sqio", "config.toml")
}

// FindLocalPath walks from start toward the filesystem root looking for the
// nearest sqio.toml. An empty start uses the current working directory.
func FindLocalPath(start string) (string, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	start, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		path := filepath.Join(start, "sqio.toml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(start)
		if parent == start {
			return "", nil
		}
		start = parent
	}
}

// DefaultTOML returns the template written by the init command.
func DefaultTOML() string {
	return `theme = "dark"
editor = "vi"

[query]
timeout = "30s"
max_rows = 1000
format = "table"

[formatter]
dialect = "postgres"
keyword_case = "upper"
identifier_case = "lower"
indent = 2
line_width = 100

[lint]
level = "warning"
enable = []
disable = []
`
}

// mergeFile merges path into cfg when the file exists and ignores missing files.
func mergeFile(cfg *Config, path string) error {
	if _, err := os.Stat(path); err == nil {
		return mergeExistingFile(cfg, path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

// mergeExistingFile decodes a TOML file and applies only explicitly defined
// fields over the current configuration.
func mergeExistingFile(cfg *Config, path string) error {
	var fileCfg Config
	meta, err := toml.DecodeFile(path, &fileCfg)
	if err != nil {
		return err
	}
	mergeConfig(cfg, fileCfg, meta)
	return nil
}

// mergeConfig overlays src onto dst using TOML metadata so zero values in an
// omitted section do not erase defaults.
func mergeConfig(dst *Config, src Config, meta toml.MetaData) {
	if meta.IsDefined("theme") {
		dst.Theme = src.Theme
	}
	if meta.IsDefined("editor") {
		dst.Editor = src.Editor
	}
	if meta.IsDefined("query", "timeout") {
		dst.Query.Timeout = src.Query.Timeout
	}
	if meta.IsDefined("query", "max_rows") {
		dst.Query.MaxRows = src.Query.MaxRows
	}
	if meta.IsDefined("query", "format") {
		dst.Query.Format = src.Query.Format
	}
	if meta.IsDefined("formatter", "dialect") {
		dst.Formatter.Dialect = src.Formatter.Dialect
	}
	if meta.IsDefined("formatter", "keyword_case") {
		dst.Formatter.KeywordCase = src.Formatter.KeywordCase
	}
	if meta.IsDefined("formatter", "identifier_case") {
		dst.Formatter.IdentifierCase = src.Formatter.IdentifierCase
	}
	if meta.IsDefined("formatter", "indent") {
		dst.Formatter.Indent = src.Formatter.Indent
	}
	if meta.IsDefined("formatter", "line_width") {
		dst.Formatter.LineWidth = src.Formatter.LineWidth
	}
	if meta.IsDefined("lint", "level") {
		dst.Lint.Level = src.Lint.Level
	}
	if meta.IsDefined("lint", "enable") {
		dst.Lint.Enable = src.Lint.Enable
	}
	if meta.IsDefined("lint", "disable") {
		dst.Lint.Disable = src.Lint.Disable
	}
	if meta.IsDefined("connections") {
		dst.Connections = mergeConnections(dst.Connections, src.Connections)
	}
}

// mergeConnections combines connection lists by name, replacing matching base
// entries and appending new named or anonymous entries.
func mergeConnections(base, overlay []Connection) []Connection {
	merged := append([]Connection(nil), base...)
	index := make(map[string]int, len(merged))
	for i, conn := range merged {
		if conn.Name != "" {
			index[conn.Name] = i
		}
	}
	for _, conn := range overlay {
		if i, ok := index[conn.Name]; ok && conn.Name != "" {
			merged[i] = conn
			continue
		}
		if conn.Name != "" {
			index[conn.Name] = len(merged)
		}
		merged = append(merged, conn)
	}
	return merged
}

// applyEnv overlays supported SQIO_* environment variables onto cfg.
func applyEnv(cfg *Config) {
	if v := os.Getenv("SQIO_THEME"); v != "" {
		cfg.Theme = v
	}
	if v := os.Getenv("SQIO_EDITOR"); v != "" {
		cfg.Editor = v
	}
	if v := os.Getenv("SQIO_QUERY_TIMEOUT"); v != "" {
		cfg.Query.Timeout = v
	}
	if v := os.Getenv("SQIO_QUERY_FORMAT"); v != "" {
		cfg.Query.Format = v
	}
	if v := os.Getenv("SQIO_QUERY_MAX_ROWS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Query.MaxRows = n
		}
	}
}

// expandConnectionEnv resolves supported secret references in connection
// passwords without logging or exposing the resulting secret values.
func expandConnectionEnv(cfg *Config) error {
	for i := range cfg.Connections {
		resolved, err := secret.Resolve(cfg.Connections[i].Password)
		if err != nil {
			return err
		}
		cfg.Connections[i].Password = resolved
		resolved, err = secret.Resolve(cfg.Connections[i].SSHTunnel.Password)
		if err != nil {
			return err
		}
		cfg.Connections[i].SSHTunnel.Password = resolved
		resolved, err = secret.Resolve(cfg.Connections[i].SSHTunnel.JumpPassword)
		if err != nil {
			return err
		}
		cfg.Connections[i].SSHTunnel.JumpPassword = resolved
	}
	return nil
}

// Connection returns the named configured connection.
func (cfg Config) Connection(name string) (Connection, error) {
	for _, conn := range cfg.Connections {
		if conn.Name == name {
			return conn, nil
		}
	}
	return Connection{}, fmt.Errorf("connection not found: %s", name)
}
