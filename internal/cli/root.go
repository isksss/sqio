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

type globalOptions struct {
	configPath string
	quiet      bool
	noColor    bool
}

var global globalOptions

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
		newExecCommand(),
		newQueryCommand(),
		newFmtCommand(),
		newLintCommand(),
		newHistoryCommand(),
		newTablesCommand(),
		newColumnsCommand(),
		newDDLCommand(),
		newSchemaCommand(),
		newERCommand(),
		newTUICommand(),
	)
	return root
}

func loadConfig() (config.Config, error) {
	cfg, err := config.Load(global.configPath)
	if err != nil {
		return cfg, &CommandError{Type: "config", Message: err.Error(), Code: ExitInternal}
	}
	return cfg, nil
}
