package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/isksss/sqio/internal/plugin"
	"github.com/spf13/cobra"
)

func newPluginCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "plugin", Short: "external plugin operations"}
	cmd.AddCommand(newPluginListCommand(), newPluginRunCommand())
	return cmd
}

func newPluginListCommand() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list external plugins on PATH",
		RunE: func(cmd *cobra.Command, args []string) error {
			plugins, err := plugin.List("")
			if err != nil {
				return &CommandError{Type: "plugin", Message: err.Error(), Code: ExitInternal}
			}
			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(plugins)
			case "", "table":
				for _, p := range plugins {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", p.Name, p.Path)
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported plugin format: " + format, Code: ExitInternal}
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	return cmd
}

func newPluginRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "run NAME [args...]",
		Short:              "run an external plugin",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := plugin.ValidateName(name); err != nil {
				return &CommandError{Type: "plugin", Message: err.Error(), Code: ExitInternal}
			}
			pluginCmd := plugin.Run(cmd.Context(), name, args[1:], "")
			pluginCmd.Stdin = os.Stdin
			pluginCmd.Stdout = cmd.OutOrStdout()
			pluginCmd.Stderr = cmd.ErrOrStderr()
			if err := pluginCmd.Run(); err != nil {
				return &CommandError{Type: "plugin", Message: err.Error(), Code: ExitInternal}
			}
			return nil
		},
	}
	return cmd
}
