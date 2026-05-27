package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/isksss/sqio/internal/service"
	"github.com/spf13/cobra"
)

type schemaDiffOptions struct {
	from   string
	to     string
	format string
}

func newSchemaDiffCommand() *cobra.Command {
	var opts schemaDiffOptions
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "diff exported schema JSON files",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.from == "" || opts.to == "" {
				return &CommandError{Type: "input", Message: "--from and --to are required", Code: ExitInternal}
			}
			from, err := readSchemaFile(opts.from)
			if err != nil {
				return &CommandError{Type: "input", Message: err.Error(), Code: ExitInternal}
			}
			to, err := readSchemaFile(opts.to)
			if err != nil {
				return &CommandError{Type: "input", Message: err.Error(), Code: ExitInternal}
			}
			diff := service.DiffSchemas(from, to)
			switch opts.format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(diff)
			case "", "table":
				for _, change := range diff.Changes {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", change.Type, change.Table, change.Name, change.Detail)
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported schema diff format: " + opts.format, Code: ExitInternal}
			}
		},
	}
	cmd.Flags().StringVar(&opts.from, "from", "", "source schema JSON file")
	cmd.Flags().StringVar(&opts.to, "to", "", "target schema JSON file")
	cmd.Flags().StringVar(&opts.format, "format", "table", "output format")
	return cmd
}

func readSchemaFile(path string) (service.Schema, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return service.Schema{}, err
	}
	var schema service.Schema
	if err := json.Unmarshal(b, &schema); err != nil {
		return service.Schema{}, err
	}
	return schema, nil
}
