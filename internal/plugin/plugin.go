// Package plugin discovers and runs external sqio plugin executables.
package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const prefix = "sqio-plugin-"

// Plugin describes one executable plugin found on PATH.
type Plugin struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// List discovers executable files named sqio-plugin-* on PATH.
func List(pathEnv string) ([]Plugin, error) {
	if pathEnv == "" {
		pathEnv = os.Getenv("PATH")
	}
	seen := map[string]bool{}
	plugins := []Plugin{}
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasPrefix(name, prefix) {
				continue
			}
			pluginName := strings.TrimPrefix(name, prefix)
			if pluginName == "" || seen[pluginName] {
				continue
			}
			fullPath := filepath.Join(dir, name)
			info, err := entry.Info()
			if err != nil || info.Mode()&0o111 == 0 {
				continue
			}
			seen[pluginName] = true
			plugins = append(plugins, Plugin{Name: pluginName, Path: fullPath})
		}
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })
	return plugins, nil
}

// Run executes the named plugin with args.
func Run(ctx context.Context, name string, args []string, pathEnv string) *exec.Cmd {
	executable := prefix + name
	if pathEnv != "" {
		if plugins, err := List(pathEnv); err == nil {
			for _, p := range plugins {
				if p.Name == name {
					executable = p.Path
					break
				}
			}
		}
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	if pathEnv != "" {
		cmd.Env = append(os.Environ(), "PATH="+pathEnv)
	}
	return cmd
}

// ValidateName rejects plugin names that could escape the sqio-plugin-* naming
// convention.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, "-") {
		return fmt.Errorf("invalid plugin name: %s", name)
	}
	return nil
}
