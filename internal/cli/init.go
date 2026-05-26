package cli

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/isksss/sqio/internal/config"
	"github.com/spf13/cobra"
)

type initOptions struct {
	global bool
}

func newInitCommand() *cobra.Command {
	var opts initOptions
	cmd := &cobra.Command{
		Use:   "init",
		Short: "create a config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "sqio.toml"
			if opts.global {
				path = config.DefaultPath()
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return &CommandError{Type: "init", Message: err.Error(), Code: ExitInternal}
				}
			}
			if err := writeConfigFile(path); err != nil {
				return err
			}
			if !global.quiet {
				_, err := cmd.OutOrStdout().Write([]byte(path + "\n"))
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&opts.global, "global", "g", false, "create global config file")
	return cmd
}

func writeConfigFile(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return &CommandError{Type: "init", Message: "config file already exists: " + path, Code: ExitInternal}
		}
		return &CommandError{Type: "init", Message: err.Error(), Code: ExitInternal}
	}
	defer file.Close()
	if _, err := file.WriteString(config.DefaultTOML()); err != nil {
		return &CommandError{Type: "init", Message: err.Error(), Code: ExitInternal}
	}
	return nil
}
