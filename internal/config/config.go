// Package config loads sqio configuration from defaults, TOML files, and
// environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
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

// SSHTunnel describes optional SSH forwarding settings for a connection.
type SSHTunnel struct {
	Enabled    bool   `toml:"enabled"`
	Host       string `toml:"host"`
	Port       int    `toml:"port"`
	User       string `toml:"user"`
	Password   string `toml:"password"`
	PrivateKey string `toml:"private_key"`
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
		expandConnectionEnv(&cfg)
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
	expandConnectionEnv(&cfg)
	return cfg, nil
}

// TimeoutDuration parses the query timeout string into a time.Duration.
func TimeoutDuration(cfg Config) (time.Duration, error) {
	return time.ParseDuration(cfg.Query.Timeout)
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

// expandConnectionEnv resolves env: references in connection passwords without
// logging or exposing the resulting secret values.
func expandConnectionEnv(cfg *Config) {
	for i := range cfg.Connections {
		if strings.HasPrefix(cfg.Connections[i].Password, "env:") {
			cfg.Connections[i].Password = os.Getenv(strings.TrimPrefix(cfg.Connections[i].Password, "env:"))
		}
		if strings.HasPrefix(cfg.Connections[i].SSHTunnel.Password, "env:") {
			cfg.Connections[i].SSHTunnel.Password = os.Getenv(strings.TrimPrefix(cfg.Connections[i].SSHTunnel.Password, "env:"))
		}
	}
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
