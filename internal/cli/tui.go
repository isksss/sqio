package cli

import (
	"context"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/isksss/sqio/internal/service"
	"github.com/isksss/sqio/internal/tui"
	"github.com/spf13/cobra"
)

type tuiProgram interface {
	Run() (tea.Model, error)
}

var newTUIProgram = func(model tea.Model) tuiProgram {
	return tea.NewProgram(model, tea.WithAltScreen())
}

// newTUICommand creates the command that launches the Bubble Tea interface.
func newTUICommand() *cobra.Command {
	var opts connectionOptions
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "open interactive TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if _, ok := os.LookupEnv("NO_COLOR"); ok {
				global.noColor = true
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			metadata, metadataCleanup, err := metadataService(ctx, cfg, opts)
			if err != nil {
				return err
			}
			defer metadataCleanup()
			driver, dsn, execCleanup, err := prepareConnection(cmd.Context(), cfg, opts)
			if err != nil {
				return err
			}
			if execCleanup != nil {
				defer execCleanup()
			}
			execOpts := service.ExecOptions{Format: cfg.Query.Format, MaxRows: cfg.Query.MaxRows, Driver: driver, DSN: dsn}
			program := newTUIProgram(tui.NewWithServices(cfg, metadata, service.Executor{}, execOpts, global.noColor))
			if _, err := program.Run(); err != nil {
				return &CommandError{Type: "tui", Message: err.Error(), Code: ExitInternal}
			}
			return nil
		},
	}
	addConnectionFlags(cmd.Flags(), &opts)
	return cmd
}
