package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/isksss/sqio/internal/config"
	"github.com/isksss/sqio/internal/service"
	"github.com/spf13/cobra"
)

type metadataOptions struct {
	connectionOptions
	format    string
	out       string
	table     string
	clipboard bool
}

func newTablesCommand() *cobra.Command {
	var opts metadataOptions
	cmd := &cobra.Command{
		Use:   "tables",
		Short: "list tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			metadata, cleanup, err := metadataService(ctx, cfg, opts.connectionOptions)
			if err != nil {
				return err
			}
			defer cleanup()
			tables, err := metadata.Tables(ctx)
			if err != nil {
				return &CommandError{Type: "metadata", Message: err.Error(), Code: ExitInternal}
			}
			w, closeOut, err := outputTarget(cmd.OutOrStdout(), opts.out)
			if err != nil {
				return err
			}
			defer closeOut()
			if opts.format == "json" {
				return json.NewEncoder(w).Encode(tables)
			}
			for _, table := range tables {
				fmt.Fprintln(w, table.Name)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.format, "format", "table", "output format")
	cmd.Flags().StringVar(&opts.out, "out", "", "output file")
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	return cmd
}

func newColumnsCommand() *cobra.Command {
	var opts metadataOptions
	cmd := &cobra.Command{
		Use:   "columns",
		Short: "list columns",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if opts.table == "" {
				return &CommandError{Type: "input", Message: "--table is required", Code: ExitInternal}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			metadata, cleanup, err := metadataService(ctx, cfg, opts.connectionOptions)
			if err != nil {
				return err
			}
			defer cleanup()
			columns, err := metadata.Columns(ctx, opts.table)
			if err != nil {
				return &CommandError{Type: "metadata", Message: err.Error(), Code: ExitInternal}
			}
			w, closeOut, err := outputTarget(cmd.OutOrStdout(), opts.out)
			if err != nil {
				return err
			}
			defer closeOut()
			if opts.format == "json" {
				return json.NewEncoder(w).Encode(columns)
			}
			for _, column := range columns {
				nullable := "not null"
				if column.Nullable {
					nullable = "nullable"
				}
				flags := nullable
				if column.Primary {
					flags += " pk"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", column.Name, column.Type, flags)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.table, "table", "", "table name")
	cmd.Flags().StringVar(&opts.format, "format", "table", "output format")
	cmd.Flags().StringVar(&opts.out, "out", "", "output file")
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	return cmd
}

func newDDLCommand() *cobra.Command {
	var opts metadataOptions
	cmd := &cobra.Command{
		Use:   "ddl",
		Short: "print table DDL",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if opts.table == "" {
				return &CommandError{Type: "input", Message: "--table is required", Code: ExitInternal}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			metadata, cleanup, err := metadataService(ctx, cfg, opts.connectionOptions)
			if err != nil {
				return err
			}
			defer cleanup()
			ddl, err := metadata.DDL(ctx, opts.table)
			if err != nil {
				return &CommandError{Type: "metadata", Message: err.Error(), Code: ExitInternal}
			}
			w, closeOut, err := outputTarget(cmd.OutOrStdout(), opts.out)
			if err != nil {
				return err
			}
			defer closeOut()
			fmt.Fprintln(w, ddl)
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.table, "table", "", "table name")
	cmd.Flags().StringVar(&opts.out, "out", "", "output file")
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	return cmd
}

func newSchemaCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "schema", Short: "schema operations"}
	cmd.AddCommand(newSchemaExportCommand())
	return cmd
}

func newSchemaExportCommand() *cobra.Command {
	var opts metadataOptions
	cmd := &cobra.Command{
		Use:   "export",
		Short: "export schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			metadata, cleanup, err := metadataService(ctx, cfg, opts.connectionOptions)
			if err != nil {
				return err
			}
			defer cleanup()
			schema, err := metadata.Schema(ctx)
			if err != nil {
				return &CommandError{Type: "metadata", Message: err.Error(), Code: ExitInternal}
			}
			w, closeOut, err := outputTarget(cmd.OutOrStdout(), opts.out)
			if err != nil {
				return err
			}
			defer closeOut()
			switch strings.ToLower(opts.format) {
			case "", "json":
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(schema)
			default:
				return &CommandError{Type: "output", Message: "unsupported schema format: " + opts.format, Code: ExitInternal}
			}
		},
	}
	cmd.Flags().StringVar(&opts.format, "format", "json", "output format")
	cmd.Flags().StringVar(&opts.out, "out", "", "output file")
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	return cmd
}

func newERCommand() *cobra.Command {
	var opts metadataOptions
	cmd := &cobra.Command{
		Use:   "er",
		Short: "export ER diagram",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			if opts.format != "" && opts.format != "mermaid" {
				return &CommandError{Type: "output", Message: "unsupported ER format: " + opts.format, Code: ExitInternal}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			metadata, cleanup, err := metadataService(ctx, cfg, opts.connectionOptions)
			if err != nil {
				return err
			}
			defer cleanup()
			diagram, err := metadata.MermaidER(ctx)
			if err != nil {
				return &CommandError{Type: "metadata", Message: err.Error(), Code: ExitInternal}
			}
			if opts.clipboard {
				if err := clipboard.WriteAll(diagram); err != nil {
					return &CommandError{Type: "clipboard", Message: err.Error(), Code: ExitInternal}
				}
				return nil
			}
			w, closeOut, err := outputTarget(cmd.OutOrStdout(), opts.out)
			if err != nil {
				return err
			}
			defer closeOut()
			fmt.Fprint(w, diagram)
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.format, "format", "mermaid", "output format")
	cmd.Flags().StringVar(&opts.out, "out", "", "output file")
	cmd.Flags().BoolVar(&opts.clipboard, "clipboard", false, "copy ER diagram to clipboard")
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	return cmd
}

func metadataService(ctx context.Context, cfg config.Config, opts connectionOptions) (service.MetadataService, func(), error) {
	driver, dsn, cleanup, err := prepareConnection(ctx, cfg, opts)
	if err != nil {
		return service.MetadataService{}, nil, err
	}
	if cleanup == nil {
		cleanup = func() {}
	}
	if driver == "" && dsn == "" {
		return service.NewMetadataService(), cleanup, nil
	}
	return service.NewConnectedMetadataService(driver, dsn), cleanup, nil
}

func outputTarget(defaultWriter interface {
	Write([]byte) (int, error)
}, path string) (interface {
	Write([]byte) (int, error)
}, func(), error) {
	if path == "" {
		return defaultWriter, func() {}, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, nil, &CommandError{Type: "output", Message: err.Error(), Code: ExitInternal}
	}
	return file, func() { _ = file.Close() }, nil
}
