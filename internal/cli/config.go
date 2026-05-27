package cli

import (
	"encoding/json"
	"fmt"

	"github.com/isksss/sqio/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "configuration operations"}
	cmd.AddCommand(newConfigValidateCommand())
	return cmd
}

func newConfigValidateCommand() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "validate configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			issues := config.Validate(cfg)
			switch format {
			case "json":
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(issues); err != nil {
					return err
				}
			case "", "table":
				for _, issue := range issues {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", issue.Path, issue.Message)
				}
				if len(issues) == 0 && !global.quiet {
					fmt.Fprintln(cmd.OutOrStdout(), "ok")
				}
			default:
				return &CommandError{Type: "output", Message: "unsupported config format: " + format, Code: ExitInternal}
			}
			if len(issues) > 0 {
				return &CommandError{Type: "config", Message: "configuration validation failed", Code: ExitInternal}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	return cmd
}
