// Package cli wires Cobra commands to sqio services and terminal I/O.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/isksss/sqio/internal/config"
	"github.com/spf13/cobra"
)

// globalOptions stores root-level flags shared by all commands.
type globalOptions struct {
	configPath string
	quiet      bool
	noColor    bool
}

// global holds the current invocation's root-level options.
var global globalOptions

// Execute builds and runs the root command with signal-aware context handling.
func Execute() error {
	root := newRootCommand()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	root.SetContext(ctx)
	if err := root.Execute(); err != nil {
		var commandErr *CommandError
		if !global.quiet && (!errors.As(err, &commandErr) || commandErr.Type != "lint_issue") {
			fmt.Fprintln(os.Stderr, StructuredError(err))
		}
		return err
	}
	return nil
}

// newRootCommand creates the top-level sqio command and registers subcommands.
func newRootCommand() *cobra.Command {
	global = globalOptions{}
	root := &cobra.Command{
		Use:           "sqio",
		Short:         "SQL + I/O database client",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&global.configPath, "config", "", "config file path")
	root.PersistentFlags().BoolVar(&global.quiet, "quiet", false, "suppress non-result output")
	root.PersistentFlags().BoolVar(&global.noColor, "no-color", false, "disable colored output")
	root.AddCommand(
		newInitCommand(),
		newConfigCommand(),
		newConnCommand(),
		newCompleteCommand(),
		newExecCommand(),
		newExplainCommand(),
		newQueryCommand(),
		newFmtCommand(),
		newLintCommand(),
		newHistoryCommand(),
		newTablesCommand(),
		newSchemasCommand(),
		newColumnsCommand(),
		newIndexesCommand(),
		newRolesCommand(),
		newGrantsCommand(),
		newDDLCommand(),
		newSchemaCommand(),
		newERCommand(),
		newDumpCommand(),
		newEditCommand(),
		newLoadCommand(),
		newMigrateCommand(),
		newPluginCommand(),
		newTUICommand(),
	)
	return root
}

// loadConfig loads configuration using the active global --config flag and wraps
// configuration failures in a CLI error.
func loadConfig() (config.Config, error) {
	cfg, err := config.Load(global.configPath)
	if err != nil {
		return cfg, &CommandError{Type: "config", Message: err.Error(), Code: ExitInternal}
	}
	return cfg, nil
}
